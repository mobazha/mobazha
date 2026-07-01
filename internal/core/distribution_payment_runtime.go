package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	evm "github.com/mobazha/mobazha3.0/internal/chains/evm"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	pkgEVM "github.com/mobazha/mobazha3.0/pkg/evm"
	"github.com/mobazha/mobazha3.0/pkg/relay"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type distributionManagedEVMSigner struct {
	keys contracts.KeyProvider
}

func (s distributionManagedEVMSigner) SignManagedManagedEscrowTransaction(
	ctx context.Context,
	request distribution.ManagedEVMSignRequest,
) (common.Address, []byte, error) {
	if err := ctx.Err(); err != nil {
		return common.Address{}, nil, err
	}
	if s.keys == nil {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: key provider unavailable")
	}
	if request.Chain == "" || request.ChainID == 0 || request.ManagedEscrowAddress == (common.Address{}) ||
		request.Purpose != distribution.ManagedEVMSignManagedEscrowTransaction || strings.TrimSpace(request.CorrelationID) == "" {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: valid chain, ManagedEscrow, purpose, and correlation ID are required")
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
	key, err := s.keys.EVMMasterKey()
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: load EVM key: %w", err)
	}
	ecdsaKey, err := crypto.ToECDSA(key.Serialize())
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: convert EVM key: %w", err)
	}
	address := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	localOwner := false
	seen := make(map[common.Address]struct{}, len(request.Owners))
	for _, owner := range request.Owners {
		if owner == (common.Address{}) {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: zero owner is forbidden")
		}
		if _, exists := seen[owner]; exists {
			return common.Address{}, nil, fmt.Errorf("distribution EVM signer: duplicate owner %s", owner.Hex())
		}
		seen[owner] = struct{}{}
		localOwner = localOwner || owner == address
	}
	if !localOwner {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: local owner %s is outside the authorized owner set", address.Hex())
	}
	signature, err := crypto.Sign(request.Digest[:], ecdsaKey)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: sign digest: %w", err)
	}
	signature[64] += 27
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
	_ distribution.ManagedEVMSigner          = distributionManagedEVMSigner{}
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
