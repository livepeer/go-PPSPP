package core

import (
	"flag"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/glog"
	libp2ppeer "github.com/libp2p/go-libp2p-peer"
	ps "github.com/libp2p/go-libp2p-peerstore"
)

// TestNetworkHandshake tests a handshake between two peers on two different ports
func TestNetworkHandshake(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// This is the bootstrap part -- set up the peers, exchange IDs/addrs, and
	// connect them in one thread.
	rand.Seed(666)
	port1 := rand.Intn(100) + 10000
	port2 := port1 + 1
	p1 := NewPeer(port1)
	p2 := NewPeer(port2)
	peerExchangeIDAddr(p1, p2)
	sid := SwarmID(8)
	glog.Infof("Handshake between %s and %s on swarm %v\n", p1.id(), p2.id(), sid)
	p1.AddSwarm(sid)
	p2.AddSwarm(sid)

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
}

func startNetworkHandshake(t *testing.T, p *Peer, remote libp2ppeer.ID, sid SwarmID, done chan bool) {
	p.AddSwarm(sid)

	// kick off the handshake
	err := p.startHandshake(remote, sid)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(3 * time.Second)

	checkState(t, sid, p, remote, Ready)

	done <- true
}

func waitNetworkHandshake(t *testing.T, p *Peer, remote libp2ppeer.ID, sid SwarmID, done chan bool) {
	time.Sleep(3 * time.Second)
	checkState(t, sid, p, remote, Ready)

	done <- true
}

func closeNetworkHandshake(t *testing.T, p *Peer, remote libp2ppeer.ID, sid SwarmID, done chan bool) {
	err := p.sendClosingHandshake(remote, sid)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(3 * time.Second)
	checkNoChannel(t, sid, p, remote)

	done <- true
}

func waitCloseNetworkHandshake(t *testing.T, p *Peer, remote libp2ppeer.ID, sid SwarmID, done chan bool) {
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
func checkState(t *testing.T, sid SwarmID, p *Peer, remote libp2ppeer.ID, state ProtocolState) {
	foundState, err := p.ProtocolState(sid, remote)
	if err != nil {
		t.Errorf("could not get state for %v: %v", p.id(), err)
	}
	if foundState != state {
		t.Errorf("%v state=%v, not %v after handshake", p.id(), foundState, state)
	}
}

// checkNoChannel checks that peer p does not have a channel for swarm sid for the remote peer
func checkNoChannel(t *testing.T, sid SwarmID, p *Peer, remote libp2ppeer.ID) {
	// This is a bit of a hacky round-about way to check that there is not channel
	// We should write a function with receiver (p *Peer) that actually checks if the channel exists
	// in the Peer object, but the ProtocolState function essentially does that already.
	foundState, err := p.ProtocolState(sid, remote)
	if !(foundState == Unknown && err != nil) {
		t.Errorf("%v found a channel for sid=%v, remote=%v", p.id(), sid, remote)
	}
}
