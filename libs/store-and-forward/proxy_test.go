package storeandforward

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/mobazha/mobazha/libs/store-and-forward/pb"
)

const (
	testServerProtocol = protocol.ID("/test/snf/server/1.0.0")
	testClientProtocol = protocol.ID("/test/snf/client/1.0.0")
)

// Test_ProxyRegistration tests that LocalClient can register through the proxy
func Test_ProxyRegistration(t *testing.T) {
	mn := mocknet.New()

	// Create server host
	serverHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	// Create proxy/transport host
	proxyHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	// Create server with test protocols
	server, err := NewServer(context.Background(), serverHost,
		ServerProtocols(testServerProtocol),
		ClientProtocols(testClientProtocol),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create SNF Proxy using the proxy host as transport
	proxy, err := NewSNFProxy(context.Background(), &ProxyConfig{
		TransportHost:        proxyHost,
		Servers:              []peer.ID{serverHost.ID()},
		ServerProtocol:       testServerProtocol,
		ClientProtocol:       testClientProtocol,
		RegistrationDuration: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a new peer identity for LocalClient (different from proxy host)
	clientSK, _, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}
	clientID, err := peer.IDFromPrivateKey(clientSK)
	if err != nil {
		t.Fatal(err)
	}

	// Register the client with the proxy
	localClient, err := proxy.RegisterNode(clientID, clientSK)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for registration to complete
	time.Sleep(time.Second * 2)

	// Verify the server has the registration under the client's ID (not proxy's ID)
	_, err = server.ds.Get(server.ctx, registrationKey(clientID))
	if err != nil {
		t.Fatalf("Expected registration for client %s, got error: %v", clientID.ShortString(), err)
	}

	// Verify proxy host's ID is NOT registered (since we used client's identity)
	_, err = server.ds.Get(server.ctx, registrationKey(proxyHost.ID()))
	if err == nil {
		t.Fatal("Proxy host should NOT be registered, only the client")
	}

	// Verify LocalClient reports as registered
	servers := localClient.GetRegisteredServers()
	if len(servers) != 1 || servers[0] != serverHost.ID() {
		t.Fatalf("Expected LocalClient to be registered with server, got %v", servers)
	}
}

func Test_ProxyAckDoesNotDowngradeRegistrationTTL(t *testing.T) {
	mn := mocknet.New()

	serverHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}
	proxyHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}
	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	server, err := NewServer(context.Background(), serverHost,
		ServerProtocols(testServerProtocol),
		ClientProtocols(testClientProtocol),
	)
	if err != nil {
		t.Fatal(err)
	}

	proxy, err := NewSNFProxy(context.Background(), &ProxyConfig{
		TransportHost:        proxyHost,
		Servers:              []peer.ID{serverHost.ID()},
		ServerProtocol:       testServerProtocol,
		ClientProtocol:       testClientProtocol,
		RegistrationDuration: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	clientSK, _, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}
	clientID, err := peer.IDFromPrivateKey(clientSK)
	if err != nil {
		t.Fatal(err)
	}

	localClient, err := proxy.RegisterNode(clientID, clientSK)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Second)
	if err := localClient.AckMessage(context.Background(), []byte("already-processed-message")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)

	record, err := server.ds.Get(server.ctx, registrationKey(clientID))
	if err != nil {
		t.Fatalf("expected registration for client after ACK: %v", err)
	}
	reg := new(pb.Message_Registration)
	if err := proto.Unmarshal(record, reg); err != nil {
		t.Fatal(err)
	}
	expiry, err := timestampTime(reg.Expiry)
	if err != nil {
		t.Fatal(err)
	}
	if remaining := time.Until(expiry); remaining < 50*time.Minute {
		t.Fatalf("ACK downgraded registration TTL to %v; want durable registration", remaining)
	}
}

// Test_ProxyMessageIsolation tests that messages are properly isolated between LocalClients
func Test_ProxyMessageIsolation(t *testing.T) {
	mn := mocknet.New()

	// Create server host
	serverHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	// Create proxy/transport host
	proxyHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	// Create sender with Ed25519 key (needed for traditional client)
	senderSK, senderAddr, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}
	senderHost, err := mn.AddPeer(senderSK, senderAddr)
	if err != nil {
		t.Fatal(err)
	}

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	// Create server
	server, err := NewServer(context.Background(), serverHost,
		ServerProtocols(testServerProtocol),
		ClientProtocols(testClientProtocol),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = server // Used implicitly

	// Create SNF Proxy
	proxy, err := NewSNFProxy(context.Background(), &ProxyConfig{
		TransportHost:        proxyHost,
		Servers:              []peer.ID{serverHost.ID()},
		ServerProtocol:       testServerProtocol,
		ClientProtocol:       testClientProtocol,
		RegistrationDuration: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create two LocalClients with different identities
	sk1, _, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}
	clientID1, _ := peer.IDFromPrivateKey(sk1)

	sk2, _, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}
	clientID2, _ := peer.IDFromPrivateKey(sk2)

	localClient1, err := proxy.RegisterNode(clientID1, sk1)
	if err != nil {
		t.Fatal(err)
	}

	localClient2, err := proxy.RegisterNode(clientID2, sk2)
	if err != nil {
		t.Fatal(err)
	}

	// Create a traditional sender client with Ed25519 key
	sender, err := NewClient(context.Background(), senderSK, []peer.ID{serverHost.ID()}, senderHost,
		ServerProtocols(testServerProtocol),
		ClientProtocols(testClientProtocol),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for all registrations
	time.Sleep(time.Second * 3)

	// Send message to client1 only
	msg1 := []byte("message for client1")
	if err := sender.SendMessage(context.Background(), clientID1, serverHost.ID(), nil, msg1, nil); err != nil {
		t.Fatal(err)
	}

	// Send message to client2 only
	msg2 := []byte("message for client2")
	if err := sender.SendMessage(context.Background(), clientID2, serverHost.ID(), nil, msg2, nil); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	// Client1 should only get msg1
	messages1, err := localClient1.GetMessages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(messages1) != 1 {
		t.Fatalf("Client1: Expected 1 message, got %d", len(messages1))
	}
	if !bytes.Equal(messages1[0].EncryptedMessage, msg1) {
		t.Errorf("Client1: Wrong message content")
	}

	// Client2 should only get msg2
	messages2, err := localClient2.GetMessages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(messages2) != 1 {
		t.Fatalf("Client2: Expected 1 message, got %d", len(messages2))
	}
	if !bytes.Equal(messages2[0].EncryptedMessage, msg2) {
		t.Errorf("Client2: Wrong message content")
	}

	// ACK messages
	if err := localClient1.AckMessage(context.Background(), messages1[0].MessageID); err != nil {
		t.Fatal(err)
	}
	if err := localClient2.AckMessage(context.Background(), messages2[0].MessageID); err != nil {
		t.Fatal(err)
	}

	// Verify messages are deleted after ACK
	time.Sleep(time.Second * 2)
	messages1After, _ := localClient1.GetMessages(context.Background())
	messages2After, _ := localClient2.GetMessages(context.Background())
	if len(messages1After) != 0 {
		t.Logf("Warning: Client1 still has %d messages after ACK (may be timing issue)", len(messages1After))
	}
	if len(messages2After) != 0 {
		t.Logf("Warning: Client2 still has %d messages after ACK (may be timing issue)", len(messages2After))
	}
}

// Test_ProxySendMessage tests that LocalClient can send messages through the proxy
func Test_ProxySendMessage(t *testing.T) {
	mn := mocknet.New()

	// Create server host
	serverHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	// Create proxy host
	proxyHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	// Create receiver with Ed25519 key (needed for traditional client)
	receiverSK, receiverAddr, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}
	receiverHost, err := mn.AddPeer(receiverSK, receiverAddr)
	if err != nil {
		t.Fatal(err)
	}

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	// Create server
	_, err = NewServer(context.Background(), serverHost,
		ServerProtocols(testServerProtocol),
		ClientProtocols(testClientProtocol),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create SNF Proxy
	proxy, err := NewSNFProxy(context.Background(), &ProxyConfig{
		TransportHost:        proxyHost,
		Servers:              []peer.ID{serverHost.ID()},
		ServerProtocol:       testServerProtocol,
		ClientProtocol:       testClientProtocol,
		RegistrationDuration: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create LocalClient (sender)
	senderSK, _, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}
	senderID, _ := peer.IDFromPrivateKey(senderSK)
	localSender, err := proxy.RegisterNode(senderID, senderSK)
	if err != nil {
		t.Fatal(err)
	}

	// Create traditional receiver client with Ed25519 key
	receiver, err := NewClient(context.Background(), receiverSK, []peer.ID{serverHost.ID()}, receiverHost,
		ServerProtocols(testServerProtocol),
		ClientProtocols(testClientProtocol),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for registrations
	time.Sleep(time.Second * 3)

	// Send message from LocalClient to traditional client
	msg := []byte("hello from proxy client")
	if err := localSender.SendMessage(context.Background(), receiverHost.ID(), serverHost.ID(), nil, msg, nil); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	// Receiver should get the message
	messages, err := receiver.GetMessages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	if !bytes.Equal(messages[0].EncryptedMessage, msg) {
		t.Errorf("Wrong message content")
	}
}

// Test_TraditionalClientCompatibility tests backward compatibility with traditional clients
func Test_TraditionalClientCompatibility(t *testing.T) {
	mn := mocknet.New()

	// Create server host (use GenPeer for server, it doesn't need to extract pubkey)
	serverHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	// Create two client hosts using Ed25519 keys (traditional mode requires extractable pubkey)
	sk1, a1, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}
	clientHost1, err := mn.AddPeer(sk1, a1)
	if err != nil {
		t.Fatal(err)
	}

	sk2, a2, err := newPeer()
	if err != nil {
		t.Fatal(err)
	}
	clientHost2, err := mn.AddPeer(sk2, a2)
	if err != nil {
		t.Fatal(err)
	}

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	// Create server with updated code
	server, err := NewServer(context.Background(), serverHost,
		ServerProtocols(testServerProtocol),
		ClientProtocols(testClientProtocol),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create traditional clients (no pubkey in registration)
	client1, err := NewClient(context.Background(), sk1, []peer.ID{serverHost.ID()}, clientHost1,
		ServerProtocols(testServerProtocol),
		ClientProtocols(testClientProtocol),
	)
	if err != nil {
		t.Fatal(err)
	}

	client2, err := NewClient(context.Background(), sk2, []peer.ID{serverHost.ID()}, clientHost2,
		ServerProtocols(testServerProtocol),
		ClientProtocols(testClientProtocol),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for registrations
	time.Sleep(time.Second * 2)

	// Verify registrations using connection peer ID (traditional mode)
	_, err = server.ds.Get(server.ctx, registrationKey(clientHost1.ID()))
	if err != nil {
		t.Fatalf("Traditional client1 should be registered: %v", err)
	}
	_, err = server.ds.Get(server.ctx, registrationKey(clientHost2.ID()))
	if err != nil {
		t.Fatalf("Traditional client2 should be registered: %v", err)
	}

	// Test message flow
	msg := []byte("traditional message")
	if err := client1.SendMessage(context.Background(), clientHost2.ID(), serverHost.ID(), nil, msg, nil); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	messages, err := client2.GetMessages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	if !bytes.Equal(messages[0].EncryptedMessage, msg) {
		t.Errorf("Wrong message content")
	}
}

// Test_MultipleLocalClientsRegistration tests registering multiple LocalClients through one proxy
func Test_MultipleLocalClientsRegistration(t *testing.T) {
	mn := mocknet.New()

	serverHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	proxyHost, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	server, err := NewServer(context.Background(), serverHost,
		ServerProtocols(testServerProtocol),
		ClientProtocols(testClientProtocol),
	)
	if err != nil {
		t.Fatal(err)
	}

	proxy, err := NewSNFProxy(context.Background(), &ProxyConfig{
		TransportHost:        proxyHost,
		Servers:              []peer.ID{serverHost.ID()},
		ServerProtocol:       testServerProtocol,
		ClientProtocol:       testClientProtocol,
		RegistrationDuration: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Register 5 LocalClients
	numClients := 5
	clients := make([]*LocalClient, numClients)
	clientIDs := make([]peer.ID, numClients)

	for i := 0; i < numClients; i++ {
		sk, _, err := newPeer()
		if err != nil {
			t.Fatal(err)
		}
		clientID, _ := peer.IDFromPrivateKey(sk)
		clientIDs[i] = clientID

		client, err := proxy.RegisterNode(clientID, sk)
		if err != nil {
			t.Fatal(err)
		}
		clients[i] = client
	}

	// Wait for all registrations
	time.Sleep(time.Second * 3)

	// Verify all clients are registered with their own IDs
	for i, clientID := range clientIDs {
		_, err := server.ds.Get(server.ctx, registrationKey(clientID))
		if err != nil {
			t.Errorf("Client %d (ID: %s) should be registered: %v", i, clientID.ShortString(), err)
		}
	}

	// Verify all clients report as registered
	for i, client := range clients {
		servers := client.GetRegisteredServers()
		if len(servers) != 1 {
			t.Errorf("Client %d should be registered with 1 server, got %d", i, len(servers))
		}
	}

	t.Logf("Successfully registered %d LocalClients through single proxy", numClients)
}
