//go:build !private_distribution

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
	"github.com/mobazha/mobazha3.0/pkg/relay"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type distributionEVMDigestSigner struct {
	keys contracts.KeyProvider
}

func (s distributionEVMDigestSigner) SignEVMDigest(
	ctx context.Context,
	request distribution.EVMDigestSignRequest,
) (common.Address, []byte, error) {
	if err := ctx.Err(); err != nil {
		return common.Address{}, nil, err
	}
	if s.keys == nil {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: key provider unavailable")
	}
	if request.Chain == "" || request.Purpose == "" || request.CorrelationID == "" {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: chain, purpose, and correlation ID are required")
	}
	key, err := s.keys.EVMMasterKey()
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: load EVM key: %w", err)
	}
	ecdsaKey, err := crypto.ToECDSA(key.Serialize())
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: convert EVM key: %w", err)
	}
	signature, err := crypto.Sign(request.Digest[:], ecdsaKey)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("distribution EVM signer: sign digest: %w", err)
	}
	signature[64] += 27
	return crypto.PubkeyToAddress(ecdsaKey.PublicKey), signature, nil
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
	_ distribution.EVMDigestSigner           = distributionEVMDigestSigner{}
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
