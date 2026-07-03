package contracts

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/chains"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type Contracts struct {
	cfg         chains.Config
	rpcEndpoint string
	client      *ethclient.Client
}

func NewContracts(opts ...chains.Option) (*Contracts, error) {
	var cfg chains.Config
	if err := cfg.Apply(append([]chains.Option{chains.Defaults}, opts...)...); err != nil {
		return nil, err
	}

	rpcEndpoint := cfg.ChainAPIs[iwallet.ChainPolygon].MainnetRpc
	if cfg.UseTestnet {
		rpcEndpoint = cfg.ChainAPIs[iwallet.ChainPolygon].TestnetRpc
	}

	client, err := ethclient.Dial(rpcEndpoint[0])
	if err != nil {
		return nil, err
	}

	return &Contracts{
		cfg:         cfg,
		rpcEndpoint: rpcEndpoint[0],
		client:      client,
	}, nil
}

func (contracts *Contracts) GetBlockedIds() ([]peer.ID, error) {
	address := common.HexToAddress(MATIC_BAN_NODES_CONTRACT_ADDRESS)
	if contracts.cfg.UseTestnet {
		address = common.HexToAddress(MATIC_BAN_NODES_CONTRACT_ADDRESS_TESTNET)
	}
	banNodesInstance, err := NewBanNodes(address, contracts.client)
	if err != nil {
		return nil, err
	}

	blockedIds, err := banNodesInstance.GetBlockedIds(nil)
	if err != nil {
		return nil, err
	}

	var ret []peer.ID
	for _, pid := range blockedIds {
		id, err := peer.Decode(pid)
		if err != nil {
			continue
		}
		ret = append(ret, id)
	}
	return ret, nil
}
