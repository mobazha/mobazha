package net

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/repo"
	storeandforward "github.com/mobazha/mobazha3.0/libs/store-and-forward"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	ma "github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

func TestMessenger(t *testing.T) {
	mocknet := mocknet.New()

	priv1, addr1, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	h1, err := mocknet.AddPeer(priv1, addr1)
	if err != nil {
		t.Fatal(err)
	}

	priv2, addr2, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	h2, err := mocknet.AddPeer(priv2, addr2)
	if err != nil {
		t.Fatal(err)
	}

	if err := mocknet.LinkAll(); err != nil {
		t.Fatal(err)
	}

	service1 := NewNetworkService("", h1, NewBanManager(nil, nil), true)
	service2 := NewNetworkService("", h2, NewBanManager(nil, nil), true)

	db1, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db1.Close()

	db2, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	messenger1, err := NewMessenger(&MessengerConfig{
		Service: service1,
		DB:      db1,
		Privkey: priv1,
		Testnet: true,
		Context: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}
	messenger2, err := NewMessenger(&MessengerConfig{
		Service: service2,
		DB:      db2,
		Testnet: true,
		Privkey: priv2,
		Context: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan struct{})
	service2.RegisterHandler(pb.Message_PING, func(p peer.ID, msg *pb.Message) error {
		ch <- struct{}{}
		return nil
	})

	ch2 := make(chan struct{})
	service1.RegisterHandler(pb.Message_ACK, func(p peer.ID, msg *pb.Message) error {
		err := db1.Update(func(tx database.Tx) error {
			ack := new(pb.AckMessage)
			if err := msg.Payload.UnmarshalTo(ack); err != nil {
				return err
			}
			return messenger1.ProcessACK(tx, ack)
		})
		if err != nil {
			t.Error(err)
		}

		ch2 <- struct{}{}
		return nil
	})

	done := make(chan struct{})
	db1.Update(func(tx database.Tx) error {
		messenger1.ReliablySendMessage(tx, service2.host.ID(), &pb.Message{MessageID: "abc", MessageType: pb.Message_PING}, done)
		return nil
	})

	var messages []models.OutgoingMessage
	err = messenger1.db.View(func(tx database.Tx) error {
		return tx.Read().Find(&messages).Error
	})
	if err != nil {
		t.Error(err)
	}

	if len(messages) != 1 {
		t.Error("Failed to delete ACKed message from the db")
	}

	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out")
	}
	select {
	case <-ch:
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out")
	}

	messenger2.SendACK("abc", service1.host.ID())
	select {
	case <-ch2:
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out")
	}

	var messages2 []models.OutgoingMessage
	err = messenger1.db.View(func(tx database.Tx) error {
		return tx.Read().Find(&messages2).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Error(err)
	}

	if len(messages2) > 0 {
		t.Error("Failed to delete ACKed message from the db")
	}
}

func TestMessenger_retryAllMessages(t *testing.T) {
	mocknet := mocknet.New()

	priv1, addr1, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	h1, err := mocknet.AddPeer(priv1, addr1)
	if err != nil {
		t.Fatal(err)
	}

	priv2, addr2, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	h2, err := mocknet.AddPeer(priv2, addr2)
	if err != nil {
		t.Fatal(err)
	}

	if err := mocknet.LinkAll(); err != nil {
		t.Fatal(err)
	}

	service1 := NewNetworkService("", h1, NewBanManager(nil, nil), true)
	service2 := NewNetworkService("", h2, NewBanManager(nil, nil), true)

	db1, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db1.Close()

	messenger, err := NewMessenger(&MessengerConfig{
		Service: service1,
		DB:      db1,
		Privkey: priv1,
		Testnet: true,
		Context: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db1.Update(func(tx database.Tx) error {
		ping := &pb.Message{
			MessageType: pb.Message_PING,
			MessageID:   "abc",
		}
		ser, err := proto.Marshal(ping)
		if err != nil {
			return err
		}
		return tx.Save(&models.OutgoingMessage{
			ID:                "abc",
			Recipient:         service2.host.ID().String(),
			SerializedMessage: ser,
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	messenger.retryAllMessages()
	ch := make(chan struct{})
	service2.RegisterHandler(pb.Message_PING, func(p peer.ID, msg *pb.Message) error {
		ch <- struct{}{}
		return nil
	})

	select {
	case <-ch:
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out waiting for ping")
	}
}

func TestMessenger_encryptDecrypt(t *testing.T) {
	mocknet := mocknet.New()

	priv1, addr1, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	h1, err := mocknet.AddPeer(priv1, addr1)
	if err != nil {
		t.Fatal(err)
	}

	pid, err := peer.IDFromPrivateKey(priv1)
	if err != nil {
		t.Fatal(err)
	}

	service1 := NewNetworkService("", h1, NewBanManager(nil, nil), true)

	messenger, err := NewMessenger(&MessengerConfig{
		Service: service1,
		Privkey: priv1,
		Testnet: true,
		Context: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}

	cipherText, err := messenger.prepEncryptedMessage(pid, &pb.Message{
		MessageType: pb.Message_CHAT,
	})
	if err != nil {
		t.Fatal(err)
	}
	retPeer, pmes, err := messenger.decryptMessage(cipherText)
	if err != nil {
		t.Fatal(err)
	}
	if retPeer != pid {
		t.Errorf("Expected peer ID %s, got %s", pid, retPeer)
	}

	if pmes.MessageType != pb.Message_CHAT {
		t.Errorf("Expected message type %s, got %s", pb.Message_CHAT, pmes.MessageType)
	}
}

func TestMessenger_DownloadMessages(t *testing.T) {
	mocknet, err := mocknet.FullMeshLinked(1)
	if err != nil {
		t.Fatal(err)
	}

	serverOpts := []storeandforward.Option{
		storeandforward.ServerProtocols(ProtocolStoreAndForwardTestnet_Server),
		storeandforward.ClientProtocols(ProtocolStoreAndForwardTestnet_Client),
	}

	_, err = storeandforward.NewServer(context.Background(), mocknet.Hosts()[0], serverOpts...)
	if err != nil {
		t.Fatal(err)
	}

	priv1, addr1, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	peer1ID, err := peer.IDFromPrivateKey(priv1)
	if err != nil {
		t.Fatal(err)
	}

	h1, err := mocknet.AddPeer(priv1, addr1)
	if err != nil {
		t.Fatal(err)
	}

	priv2, addr2, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	h2, err := mocknet.AddPeer(priv2, addr2)
	if err != nil {
		t.Fatal(err)
	}

	service1 := NewNetworkService("", h1, NewBanManager(nil, nil), true)
	ch := make(chan struct{})
	service1.RegisterHandler(pb.Message_CHAT, func(p peer.ID, msg *pb.Message) error {
		ch <- struct{}{}
		return nil
	})

	service2 := NewNetworkService("", h2, NewBanManager(nil, nil), true)

	db1, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db1.Close()

	db2, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	if err := mocknet.LinkAll(); err != nil {
		t.Fatal(err)
	}

	messenger1, err := NewMessenger(&MessengerConfig{
		Service:    service1,
		DB:         db1,
		Privkey:    priv1,
		SNFServers: []peer.ID{mocknet.Hosts()[0].ID()},
		Testnet:    true,
		Context:    context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}

	messenger2, err := NewMessenger(&MessengerConfig{
		Service:    service2,
		DB:         db2,
		Privkey:    priv2,
		SNFServers: []peer.ID{mocknet.Hosts()[0].ID()},
		Testnet:    true,
		Context:    context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// It sucks to need to use a timeout here but we don't currently
	// have a better way to allow both Clients to authenticate and
	// register with the server.
	time.Sleep(time.Second)

	if err := mocknet.UnlinkPeers(h1.ID(), h2.ID()); err != nil {
		t.Fatal(err)
	}

	if err := mocknet.UnlinkPeers(h1.ID(), mocknet.Hosts()[0].ID()); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	err = db2.Update(func(tx database.Tx) error {
		rec := models.StoreAndForwardServers{
			PeerID:      peer1ID.String(),
			LastUpdated: time.Now(),
		}
		if err := rec.PutServers([]string{mocknet.Hosts()[0].ID().String()}); err != nil {
			return err
		}
		err := tx.Save(&rec)
		if err != nil {
			return err
		}

		return messenger2.ReliablySendMessage(tx, peer1ID, &pb.Message{MessageType: pb.Message_CHAT}, done)
	})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out")
	}

	if err := mocknet.LinkAll(); err != nil {
		t.Fatal(err)
	}

	go messenger1.downloadMessages()

	select {
	case <-ch:
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out waiting for message")
	}
}

var blackholeIP6 = net.ParseIP("100::")

func newPeer() (crypto.PrivKey, ma.Multiaddr, error) {
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	id, err := peer.IDFromPrivateKey(sk)
	if err != nil {
		return nil, nil, err
	}

	suffix := id
	if len(id) > 8 {
		suffix = id[len(id)-8:]
	}
	ip := append(net.IP{}, blackholeIP6...)
	copy(ip[net.IPv6len-len(suffix):], suffix)
	a, err := ma.NewMultiaddr(fmt.Sprintf("/ip6/%s/tcp/4242", ip))
	if err != nil {
		return nil, nil, err
	}
	return sk, a, nil
}
