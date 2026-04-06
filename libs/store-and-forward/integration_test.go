package storeandforward

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
)

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

func Test_Registration(t *testing.T) {
	mn, err := mocknet.WithNPeers(1)
	if err != nil {
		t.Fatal(err)
	}

	server, err := NewServer(context.Background(), mn.Hosts()[0])
	if err != nil {
		t.Fatal(err)
	}

	sk, a, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	clientHost, err := mn.AddPeer(sk, a)
	if err != nil {
		t.Fatal(err)
	}

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(context.Background(), sk, []peer.ID{}, clientHost)
	if err != nil {
		t.Fatal(err)
	}

	client.registerSingle(server.host.ID(), time.Hour)

	_, err = server.ds.Get(server.ctx, registrationKey(client.host.ID()))
	if err != nil {
		t.Fatal(err)
	}

	client.mtx.RLock()
	registered := client.servers[server.host.ID()]
	client.mtx.RUnlock()
	if !registered {
		t.Fatal("Registration failed")
	}
}

func Test_Messages(t *testing.T) {
	mn, err := mocknet.WithNPeers(1)
	if err != nil {
		t.Fatal(err)
	}

	server, err := NewServer(context.Background(), mn.Hosts()[0])
	if err != nil {
		t.Fatal(err)
	}

	sk1, a1, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	sk2, a2, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	h1, err := mn.AddPeer(sk1, a1)
	if err != nil {
		t.Fatal(err)
	}

	h2, err := mn.AddPeer(sk2, a2)
	if err != nil {
		t.Fatal(err)
	}

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	client1, err := NewClient(context.Background(), sk1, []peer.ID{}, h1)
	if err != nil {
		t.Fatal(err)
	}

	serverID := server.host.ID()
	client1.registerSingle(serverID, time.Hour)

	client2, err := NewClient(context.Background(), sk2, []peer.ID{}, h2)
	if err != nil {
		t.Fatal(err)
	}

	client2.registerSingle(serverID, time.Hour)

	var (
		sub    = client2.SubscribeMessages()
		encMsg = []byte("encrypted message")
	)

	if err := client1.SendMessage(context.Background(), h2.ID(), serverID, nil, encMsg, nil); err != nil {
		t.Fatal(err)
	}

	select {
	case m := <-sub.Out:
		if !bytes.Equal(m.EncryptedMessage, encMsg) {
			t.Errorf("Wrong message. Expected %s, got %s", string(encMsg), string(m.EncryptedMessage))
		}
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out waiting on sub")
	}

	messages, err := client2.GetMessages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("Wrong number of messages. Expected %d, got %d", 1, len(messages))
	}
	if !bytes.Equal(messages[0].EncryptedMessage, encMsg) {
		t.Errorf("Wrong message. Expected %s, got %s", string(encMsg), string(messages[0].EncryptedMessage))
	}

	if err := client2.AckMessage(context.Background(), messages[0].MessageID); err != nil {
		t.Fatal(err)
	}

	_, err = server.ds.Get(server.ctx, messageKey(h2.ID(), messages[0].MessageID))
	if err != datastore.ErrNotFound {
		t.Errorf("Expected ErrNotFound got %v", err)
	}

	// Make sure second message goes through.
	encMsg = []byte("encrypted message2")
	if err := client1.SendMessage(context.Background(), h2.ID(), serverID, nil, encMsg, nil); err != nil {
		t.Fatal(err)
	}

	select {
	case m := <-sub.Out:
		if !bytes.Equal(m.EncryptedMessage, encMsg) {
			t.Errorf("Wrong message. Expected %s, got %s", string(encMsg), string(m.EncryptedMessage))
		}
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out waiting on sub")
	}

	messages, err = client2.GetMessages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("Wrong number of messages. Expected %d, got %d", 1, len(messages))
	}
	if !bytes.Equal(messages[0].EncryptedMessage, encMsg) {
		t.Errorf("Wrong message. Expected %s, got %s", string(encMsg), string(messages[0].EncryptedMessage))
	}

	if err := client2.AckMessage(context.Background(), messages[0].MessageID); err != nil {
		t.Fatal(err)
	}

	_, err = server.ds.Get(server.ctx, messageKey(h2.ID(), messages[0].MessageID))
	if err != datastore.ErrNotFound {
		t.Errorf("Expected ErrNotFound got %v", err)
	}
}

func Test_Replication(t *testing.T) {
	mn := mocknet.New()
	h0, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	h1, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewServer(context.Background(), h0, ReplicationPeers(h1.ID()))
	if err != nil {
		t.Fatal(err)
	}

	server2, err := NewServer(context.Background(), h1, ReplicationPeers(h0.ID()))
	if err != nil {
		t.Fatal(err)
	}

	sk1, a1, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	sk2, a2, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}

	h2, err := mn.AddPeer(sk1, a1)
	if err != nil {
		t.Fatal(err)
	}

	h3, err := mn.AddPeer(sk2, a2)
	if err != nil {
		t.Fatal(err)
	}

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	client1, err := NewClient(context.Background(), sk1, []peer.ID{}, h2)
	if err != nil {
		t.Fatal(err)
	}

	client1.registerSingle(h0.ID(), time.Hour)

	client2, err := NewClient(context.Background(), sk2, []peer.ID{}, h3)
	if err != nil {
		t.Fatal(err)
	}

	client2.registerSingle(h0.ID(), time.Hour)

	encMsg := []byte("encrypted message")

	id := sha256.Sum256(append([]byte(h2.ID()), encMsg...))

	err = client1.SendMessage(context.Background(), h3.ID(), h0.ID(), nil, encMsg, nil)
	if err != nil {
		t.Fatal(err)
	}

	c := time.After(time.Second * 10)
	for {
		select {
		case <-c:
			t.Fatal("Timed out")
		default:
		}

		has, err := server2.ds.Has(server2.ctx, messageKey(h3.ID(), id[:]))
		if err != nil {
			t.Fatal(err)
		}
		if has {
			break
		}
	}
}
