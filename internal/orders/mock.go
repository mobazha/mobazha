package orders

import (
	"context"
	"path"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core"
	coremock "github.com/ipfs/kubo/core/mock"
	"github.com/ipfs/kubo/repo/fsrepo"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func newMockOrderProcessor() (*OrderProcessor, func(), error) {
	r, err := repo.MockRepo()
	if err != nil {
		return nil, nil, err
	}

	ipfsRepo, err := fsrepo.Open(path.Join(r.DataDir(), repo.IPFSDirName))
	if err != nil {
		return nil, nil, err
	}

	ipfsConfig, err := ipfsRepo.Config()
	if err != nil {
		return nil, nil, err
	}

	ipfsConfig.Bootstrap = nil

	var dbIdentityKey models.Key
	err = r.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
	})

	ipfsConfig.Identity, err = repo.IdentityFromKey(dbIdentityKey.Value)
	if err != nil {
		return nil, nil, err
	}

	ctx := context.Background()

	mn := mocknet.New()

	ipfsNode, err := core.NewNode(ctx, &core.BuildCfg{
		Online: true,
		Repo:   ipfsRepo,
		Host:   coremock.MockHostOption(mn),
	})
	if err != nil {
		return nil, nil, err
	}

	banManager := net.NewBanManager(nil, nil)
	service := net.NewNetworkService(ipfsNode.PeerHost, banManager, true)

	messenger, err := net.NewMessenger(&net.MessengerConfig{
		Privkey: ipfsNode.PrivateKey,
		Service: service,
		DB:      r.DB(),
		Context: ipfsNode.Context(),
	})
	if err != nil {
		return nil, nil, err
	}

	mw := multiwallet.Multiwallet{
		iwallet.CtMock: wallet.NewMockWallet(),
	}

	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		return nil, nil, err
	}

	return NewOrderProcessor(&Config{
			Identity:             ipfsNode.Identity,
			IdentityPrivateKey:   ipfsNode.PrivateKey,
			Db:                   r.DB(),
			Messenger:            messenger,
			Multiwallet:          mw,
			ExchangeRateProvider: erp,
			EventBus:             events.NewBus(),
			CalcCIDFunc:          calcMockCID,
		}), func() {
			ipfsNode.Close()
			r.DestroyRepo()
		}, nil
}

func calcMockCID(file []byte) (cid.Cid, error) {
	h, err := utils.MultihashSha256(file)
	if err != nil {
		return cid.Cid{}, err
	}
	return cid.Decode(h.B58String())
}
