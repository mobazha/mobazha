package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	evm "github.com/mobazha/mobazha/internal/chains/evm"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	pkgEVM "github.com/mobazha/mobazha/pkg/evm"
	"github.com/mobazha/mobazha/pkg/relay"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type distributionManagedEVMSigner struct {
	keys       contracts.KeyProvider
	settlement contracts.SettlementSigner
}

func (s distributionManagedEVMSigner) SignManagedSettlementTransaction(
	ctx context.Context,
	request distribution.ManagedEVMSignRequest,
) (common.Address, []byte, error) {
	return s.signManagedSettlement(ctx, request, request.EscrowAddress, distribution.ManagedEVMSignSettlementTransaction)
}

func (s distributionManagedEVMSigner) signManagedSettlement(
	ctx context.Context,
	request distribution.ManagedEVMSignRequest,
	escrowAddress common.Address,
	purpose distribution.ManagedEVMSignPurpose,
) (common.Address, []byte, error) {
	if err := ctx.Err(); err != nil {
		return common.Address{}, nil, err
	}
	if s.keys == nil && s.settlement == nil {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: key provider unavailable")
	}
	if request.Chain == "" || request.ChainID == 0 || escrowAddress == (common.Address{}) ||
		request.Purpose != purpose || strings.TrimSpace(request.CorrelationID) == "" {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: valid chain, escrow, purpose, and correlation ID are required")
	}
	if request.Digest == ([32]byte{}) {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: zero digest is forbidden")
	}
	chain, ok := pkgEVM.ChainTypeForID(request.ChainID)
	if !ok || iwallet.ChainType(chain) != request.Chain {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: chain %s does not match chain ID %d", request.Chain, request.ChainID)
	}
	if request.Threshold == 0 || request.Threshold > uint64(len(request.Owners)) || len(request.Owners) == 0 {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: invalid owner threshold")
	}
	seen := make(map[common.Address]struct{}, len(request.Owners))
	for _, owner := range request.Owners {
		if owner == (common.Address{}) {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: zero owner is forbidden")
		}
		if _, exists := seen[owner]; exists {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: duplicate owner %s", owner.Hex())
		}
		seen[owner] = struct{}{}
	}
	var address common.Address
	var signature []byte
	var err error
	if request.AttemptScope != nil {
		signer, ok := s.settlement.(contracts.EVMSettlementSigner)
		if !ok {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: attempt-scoped EVM signer unavailable")
		}
		scope := request.AttemptScope
		scopeCoin, scopeErr := iwallet.CoinInfoFromCoinType(iwallet.CoinType(scope.KeyRef.RailID))
		if scopeErr != nil || scopeCoin.Chain != request.Chain {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: attempt rail does not match chain %s", request.Chain)
		}
		address, signature, err = signer.SignEVMDigest(ctx, contracts.EVMDigestSettlementSignRequest{
			KeyRef: scope.KeyRef, OrderID: scope.OrderID, AttemptID: scope.AttemptID,
			Action: scope.Action, Sequence: scope.Sequence, TermsHash: scope.TermsHash,
			ChainID: request.ChainID, Digest: request.Digest,
		})
		if err != nil {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: %w", err)
		}
	} else {
		if s.keys == nil {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: key provider unavailable")
		}
		key, keyErr := s.keys.EVMMasterKey()
		if keyErr != nil {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: load EVM key: %w", keyErr)
		}
		ecdsaKey, keyErr := crypto.ToECDSA(key.Serialize())
		if keyErr != nil {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: convert EVM key: %w", keyErr)
		}
		address = crypto.PubkeyToAddress(ecdsaKey.PublicKey)
		signature, err = crypto.Sign(request.Digest[:], ecdsaKey)
		if err != nil {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: sign digest: %w", err)
		}
		signature[64] += 27
	}
	localOwner := false
	for _, owner := range request.Owners {
		localOwner = localOwner || owner == address
	}
	if !localOwner {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: local owner %s is outside the authorized owner set", address.Hex())
	}
	return address, signature, nil
}

type distributionEVMReaderProvider struct {
	wallets contracts.WalletOperator
}

func (p distributionEVMReaderProvider) ReaderForChain(chain iwallet.ChainType) (distribution.EVMContractReader, error) {
	client, err := p.chainClient(chain)
	if err != nil {
		return nil, err
	}
	reader, ok := client.(distribution.EVMContractReader)
	if !ok {
		return nil, fmt.Errorf("distribution EVM reader: chain client for %s does not support contract reads", chain)
	}
	return reader, nil
}

func (p distributionEVMReaderProvider) LogSubscriberForChain(chain iwallet.ChainType) (distribution.EVMLogSubscriber, error) {
	client, err := p.chainClient(chain)
	if err != nil {
		return nil, err
	}
	subscriber, ok := client.(distribution.EVMLogSubscriber)
	if !ok {
		return nil, fmt.Errorf("distribution EVM reader: chain client for %s does not support log subscriptions", chain)
	}
	return subscriber, nil
}

func (p distributionEVMReaderProvider) chainClient(chain iwallet.ChainType) (iwallet.ChainClient, error) {
	if p.wallets == nil {
		return nil, fmt.Errorf("distribution EVM reader: wallet operator unavailable")
	}
	wallet, ok := p.wallets.WalletForChain(chain)
	if !ok {
		return nil, fmt.Errorf("distribution EVM reader: wallet for chain %s is unavailable", chain)
	}
	ethWallet, ok := wallet.(*evm.ETHWallet)
	if !ok {
		return nil, fmt.Errorf("distribution EVM reader: wallet for chain %s is %T, not an EVM wallet", chain, wallet)
	}
	if ethWallet.ChainClient == nil {
		return nil, fmt.Errorf("distribution EVM reader: chain client for %s is unavailable", chain)
	}
	return ethWallet.ChainClient, nil
}

var (
	_ distribution.ManagedSettlementSigner   = distributionManagedEVMSigner{}
	_ distribution.EVMContractReaderProvider = distributionEVMReaderProvider{}
	_ distribution.EVMLogSubscriberProvider  = distributionEVMReaderProvider{}
)

func (n *MobazhaNode) distributionEVMRelayService() relay.EVMRelayService {
	if n == nil {
		return nil
	}
	if n.hostService != nil {
		return n.hostService.GetEVMRelayService()
	}
	if url := strings.TrimSpace(n.relayAPIURL); url != "" {
		return relay.NewHTTPPlatformRelay(url, relay.BearerFromConfigOrEnv(n.relayAPIBearer))
	}
	return nil
}
