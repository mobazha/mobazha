package orders

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peerstore"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
	corecontracts "github.com/mobazha/mobazha-core/contracts"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func newMockOrderProcessor() (*OrderProcessor, func(), error) {
	r, err := repo.MockRepo()
	if err != nil {
		return nil, nil, err
	}

	var dbIdentityKey models.Key
	err = r.DB().View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "identity").First(&dbIdentityKey).Error
	})
	if err != nil {
		return nil, nil, err
	}

	privKey, peerID, err := repo.PrivKeyAndPeerIDFromKey(dbIdentityKey.Value)
	if err != nil {
		return nil, nil, err
	}

	mn := mocknet.New()
	h, err := mn.AddPeer(privKey, nil)
	if err != nil {
		return nil, nil, err
	}
	mockListenAddr, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/10000")
	h.Peerstore().AddAddrs(h.ID(), []ma.Multiaddr{mockListenAddr}, peerstore.PermanentAddrTTL)

	banManager := net.NewBanManager(nil, nil)
	service := net.NewNetworkService("", h, banManager, true)

	messenger, err := net.NewMessenger(&net.MessengerConfig{
		Privkey: privKey,
		Service: service,
		DB:      r.DB(),
		Context: context.Background(),
	})
	if err != nil {
		return nil, nil, err
	}

	mw := multiwallet.Multiwallet{
		iwallet.ChainMock: wallet.NewMockWallet(),
	}

	erp, err := wallet.NewMockExchangeRates()
	if err != nil {
		return nil, nil, err
	}

	signer, err := corecontracts.NewKeyPairSignerFromMarshaledKey(dbIdentityKey.Value)
	if err != nil {
		return nil, nil, err
	}

	return NewOrderProcessor(&Config{
			Identity:             peerID,
			Signer:               signer,
			Db:                   r.DB(),
			Messenger:            messenger,
			Multiwallet:          &mw,
			ExchangeRateProvider: erp,
			EventBus:             events.NewBus(),
			CalcCIDFunc:          calcMockCID,
			FeatureManager:       config.GetGlobalFeatureManager(),
		}), func() {
			h.Close()
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
