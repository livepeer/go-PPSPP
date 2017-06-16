package core

import (
	"log"
	"math/rand"
	"testing"
	"time"

	peer "github.com/libp2p/go-libp2p-peer"
	ps "github.com/libp2p/go-libp2p-peerstore"
)

// TestNetworkHandshake tests a handshake between two peers on two different ports
func TestNetworkHandshake(t *testing.T) {
	// This is the bootstrap part -- set up the peers, exchange IDs/addrs, and
	// connect them in one thread.
	rand.Seed(666)
	port1 := rand.Intn(100) + 10000
	port2 := port1 + 1
	p1 := NewPeer(port1)
	p2 := NewPeer(port2)
	peerExchangeIDAddr(p1, p2)
	sid := SwarmID(8)
	log.Printf("Handshake between %s and %s on swarm %v\n", p1.id(), p2.id(), sid)
	p1.AddSwarm(sid)
	p2.AddSwarm(sid)

	// ws1, err1 := p1.Connect(p2.h.ID())
	// if err1 != nil {
	// 	t.Fatal(err1)
	// }
	// _, err2 := p2.Connect(p1.h.ID())
	// if err2 != nil {
	// 	t.Fatal(err2)
	// }

	// First phase: start handshake in one thread, wait in the other
	// Each thread checks that state is moved to ready before setting the done channel
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	go startNetworkHandshake(t, p1, p2.id(), sid, done1)
	go waitNetworkHandshake(t, p2, p1.id(), sid, done2)
	<-done1
	<-done2

	// Second phase: close the handshake in one thread, wait in the other.
	// Each thread checks that the channel is gone before setting the done channel
	done3 := make(chan bool, 1)
	done4 := make(chan bool, 1)
	go waitCloseNetworkHandshake(t, p1, p2.id(), sid, done3)
	go closeNetworkHandshake(t, p2, p1.id(), sid, done4)
	<-done3
	<-done4

	// p1.Disconnect(p2.h.ID())
	// p2.Disconnect(p1.h.ID())
}

func startNetworkHandshake(t *testing.T, p *Peer, remote peer.ID, sid SwarmID, done chan bool) {
	p.AddSwarm(sid)

	// kick off the handshake
	err := p.startHandshake(remote, sid)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(3 * time.Second)

	checkState(t, sid, p, remote, ready)

	done <- true
}

func waitNetworkHandshake(t *testing.T, p *Peer, remote peer.ID, sid SwarmID, done chan bool) {
	time.Sleep(3 * time.Second)
	checkState(t, sid, p, remote, ready)

	done <- true
}

func closeNetworkHandshake(t *testing.T, p *Peer, remote peer.ID, sid SwarmID, done chan bool) {
	err := p.sendClosingHandshake(remote, sid)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(3 * time.Second)
	checkNoChannel(t, sid, p, remote)

	done <- true
}

func waitCloseNetworkHandshake(t *testing.T, p *Peer, remote peer.ID, sid SwarmID, done chan bool) {
	time.Sleep(3 * time.Second)

	checkNoChannel(t, sid, p, remote)

	done <- true
}

// magic exchange of peer IDs and addrs
func peerExchangeIDAddr(p1 *Peer, p2 *Peer) {
	h1 := p1.h
	h2 := p2.h
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), ps.PermanentAddrTTL)
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), ps.PermanentAddrTTL)
}

// checkState checks that the peer's ProtocolState is equal to state for swarm sid for the remote peer
func checkState(t *testing.T, sid SwarmID, p *Peer, remote peer.ID, state ProtocolState) {
	foundState, err := p.ProtocolState(sid, remote)
	if err != nil {
		t.Errorf("could not get state for %v: %v", p.id(), err)
	}
	if foundState != state {
		t.Errorf("%v state=%v, not %v after handshake", p.id(), foundState, state)
	}
}

// checkNoChannel checks that peer p does not have a channel for swarm sid for the remote peer
func checkNoChannel(t *testing.T, sid SwarmID, p *Peer, remote peer.ID) {
	// This is a bit of a hacky round-about way to check that there is not channel
	// We should write a function with receiver (p *Peer) that actually checks if the channel exists
	// in the Peer object, but the ProtocolState function essentially does that already.
	foundState, err := p.ProtocolState(sid, remote)
	if !(foundState == unknown && err != nil) {
		t.Errorf("%v found a channel for sid=%v, remote=%v", p.id(), sid, remote)
	}
}

// HANDSHAKE Tests TODO (from the RFC):
// The first datagram the initiating Peer P sends to Peer Q MUST start with a HANDSHAKE message
// Handshake message must contain:
// - a channelID, chanP, randomly chosen as specified in Section 12.1
// - the metadata of Swarm S, encoded as protocol options, as specified in Section 7. In particular, the initiating Peer P MUST include the swarm ID.
// - The capabilities of Peer P, in particular, its supported protocol versions, "Live Discard Window" (in case of a live swarm) and "Supported Messages", encoded as protocol options.
// This datagram MAY also contain some minor additional payload, e.g., HAVE messages to indicate Peer P's current progress, but it MUST NOT include any heavy payload (defined in Section 1.3), such as a DATA message.
