package core

import (
	"log"
	"math/rand"
	"testing"
	"time"

	ps "github.com/libp2p/go-libp2p-peerstore"
)

// TestNetworkHandshake tests a handshake between two peers on two different ports
func TestNetworkHandshake(t *testing.T) {
	// This is the bootstrap part -- set up the peers, exchange IDs/addrs, and
	// connect them in one thread.
	t.Error("TODO: consider doing all setup in two different go routines")
	rand.Seed(666)
	port1 := rand.Intn(100) + 10000
	port2 := port1 + 1
	p1 := NewPeer(port1)
	p2 := NewPeer(port2)
	peerExchangeIDAddr(p1, p2)
	log.Printf("This is a conversation between %s and %s\n", p1.h.ID(), p2.h.ID())
	// ws1, err1 := p1.Connect(p2.h.ID())
	// if err1 != nil {
	// 	t.Fatal(err1)
	// }
	// _, err2 := p2.Connect(p1.h.ID())
	// if err2 != nil {
	// 	t.Fatal(err2)
	// }

	// kick off the handshake
	err := p1.startHandshake(p2.h.ID())
	if err != nil {
		t.Error(err)
	}

	t.Error("TODO: instead of sleep, check that both peers are in READY state")
	time.Sleep(5 * time.Second)

	t.Error("TODO: p2.sendClosingHandshake(p1c, wrappedStream)")
	t.Error("TODO: the closing handshake does not seem to show up at p1")
	t.Error("TODO: check that the channel is closed on both peers")

	// p1.Disconnect(p2.h.ID())
	// p2.Disconnect(p1.h.ID())
}

// magic exchange of peer IDs and addrs
func peerExchangeIDAddr(p1 *Peer, p2 *Peer) {
	h1 := p1.h
	h2 := p2.h
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), ps.PermanentAddrTTL)
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), ps.PermanentAddrTTL)
}

// HANDSHAKE Tests TODO (from the RFC):
// The first datagram the initiating Peer P sends to Peer Q MUST start with a HANDSHAKE message
// Handshake message must contain:
// - a channelID, chanP, randomly chosen as specified in Section 12.1
// - the metadata of Swarm S, encoded as protocol options, as specified in Section 7. In particular, the initiating Peer P MUST include the swarm ID.
// - The capabilities of Peer P, in particular, its supported protocol versions, "Live Discard Window" (in case of a live swarm) and "Supported Messages", encoded as protocol options.
// This datagram MAY also contain some minor additional payload, e.g., HAVE messages to indicate Peer P's current progress, but it MUST NOT include any heavy payload (defined in Section 1.3), such as a DATA message.
