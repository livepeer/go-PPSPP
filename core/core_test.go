package core_test

import (
	"flag"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/glog"
	ps "github.com/libp2p/go-libp2p-peerstore"

	"bytes"

	"github.com/livepeer/go-PPSPP/core"
)

// TestNetworkHandshake tests a handshake between two peers on two different ports
func TestNetworkHandshake(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// This is the bootstrap part -- set up the peers, exchange IDs/addrs, and
	// connect them in one thread.
	sid := core.SwarmID(8)
	p1, p2 := setupTwoPeerSwarm(666, sid)
	glog.Infof("Handshake between %s and %s on swarm %v\n", p1.ID(), p2.ID(), sid)

	// First phase: start handshake in one thread, wait in the other
	// Each thread checks that state is moved to ready before setting the done channel
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	go startNetworkHandshake(t, p1, p2.ID(), sid, done1)
	go waitNetworkHandshake(t, p2, p1.ID(), sid, done2)
	<-done1
	<-done2

	// Second phase: close the handshake in one thread, wait in the other.
	// Each thread checks that the channel is gone before setting the done channel
	done3 := make(chan bool, 1)
	done4 := make(chan bool, 1)
	go waitCloseNetworkHandshake(t, p1, p2.ID(), sid, done3)
	go closeNetworkHandshake(t, p2, p1.ID(), sid, done4)
	<-done3
	<-done4
}

func setupTwoPeerSwarm(seed int64, sid core.SwarmID) (*core.Peer, *core.Peer) {
	rand.Seed(seed)
	port1 := rand.Intn(100) + 10000
	port2 := port1 + 1
	p1 := core.NewPeer(port1)
	p2 := core.NewPeer(port2)
	peerExchangeIDAddr(p1, p2)
	p1.AddSwarm(sid)
	p2.AddSwarm(sid)
	return p1, p2
}

func startNetworkHandshake(t *testing.T, p *core.Peer, remote core.PeerID, sid core.SwarmID, done chan bool) {
	p.AddSwarm(sid)

	// kick off the handshake
	err := p.StartHandshake(remote, sid)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(3 * time.Second)

	checkState(t, sid, p, remote, core.Ready)

	done <- true
}

func waitNetworkHandshake(t *testing.T, p *core.Peer, remote core.PeerID, sid core.SwarmID, done chan bool) {
	time.Sleep(3 * time.Second)
	checkState(t, sid, p, remote, core.Ready)

	done <- true
}

func closeNetworkHandshake(t *testing.T, p *core.Peer, remote core.PeerID, sid core.SwarmID, done chan bool) {
	err := p.SendClosingHandshake(remote, sid)
	if err != nil {
		t.Error(err)
	}

	time.Sleep(3 * time.Second)
	checkNoChannel(t, sid, p, remote)

	done <- true
}

func waitCloseNetworkHandshake(t *testing.T, p *core.Peer, remote core.PeerID, sid core.SwarmID, done chan bool) {
	time.Sleep(3 * time.Second)

	checkNoChannel(t, sid, p, remote)

	done <- true
}

// magic exchange of peer IDs and addrs
func peerExchangeIDAddr(p1 *core.Peer, p2 *core.Peer) {
	h1 := p1.Host()
	h2 := p2.Host()
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), ps.PermanentAddrTTL)
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), ps.PermanentAddrTTL)
}

// checkState checks that the peer's ProtocolState is equal to state for swarm sid for the remote peer
func checkState(t *testing.T, sid core.SwarmID, p *core.Peer, remote core.PeerID, state core.ProtocolState) {
	foundState, err := p.ProtocolState(sid, remote)
	if err != nil {
		t.Errorf("could not get state for %v: %v", p.ID(), err)
	}
	if foundState != state {
		t.Errorf("%v state=%v, not %v after handshake", p.ID(), foundState, state)
	}
}

// checkNoChannel checks that peer p does not have a channel for swarm sid for the remote peer
func checkNoChannel(t *testing.T, sid core.SwarmID, p *core.Peer, remote core.PeerID) {
	// This is a bit of a hacky round-about way to check that there is not channel
	// We should write a function with receiver (p *Peer) that actually checks if the channel exists
	// in the Peer object, but the ProtocolState function essentially does that already.
	foundState, err := p.ProtocolState(sid, remote)
	if !(foundState == core.Unknown && err != nil) {
		t.Errorf("%v found a channel for sid=%v, remote=%v", p.ID(), sid, remote)
	}
}

// Test NetworkDataExchange tests data exhcnage between two peers over two different ports
func TestNetworkDataExchange(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	reference := map[core.ChunkID]string{0: "The", 1: "quick", 2: "brown", 3: "fox", 4: "jumps", 5: "over", 6: "the", 7: "lazy", 8: "dog"}

	// This is the bootstrap part -- set up the peers, exchange IDs/addrs, and
	// connect them in one thread.
	sid := core.SwarmID(8)
	p1, p2 := setupTwoPeerSwarm(666, sid)
	glog.Infof("Data exchange between %s and %s on swarm %v\n", p1.ID(), p2.ID(), sid)

	// First phase: start handshake in one thread, wait in the other
	// Each thread checks that state is moved to ready before setting the done channel
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)
	go startNetworkHandshake(t, p1, p2.ID(), sid, done1)
	go waitNetworkHandshake(t, p2, p1.ID(), sid, done2)
	<-done1
	<-done2

	sendHaves(t, reference, sid, p2, p1.ID())

	time.Sleep(3 * time.Second)

	t.Error("TODO: p1 automatically sends requests for chunks it needs")
	t.Error("TODO: p2 automatically replies with data")
	t.Error("TODO: check that p1 has the right data")

	// Second phase: close the handshake in one thread, wait in the other.
	// Each thread checks that the channel is gone before setting the done channel
	done3 := make(chan bool, 1)
	done4 := make(chan bool, 1)
	go waitCloseNetworkHandshake(t, p1, p2.ID(), sid, done3)
	go closeNetworkHandshake(t, p2, p1.ID(), sid, done4)
	<-done3
	<-done4
}

func sendHaves(t *testing.T, ref map[core.ChunkID]string, s core.SwarmID, p *core.Peer, remote core.PeerID) {
	swarm, err := p.Swarm(s)
	if err != nil {
		t.Fatalf("sendHaves could not find swarm %v: %v", s, err)
	}
	for i, data := range ref {
		c := core.Chunk{ID: i, B: bytes.NewBufferString(data)}
		swarm.AddLocalChunk(i, &c)
	}
	for i, _ := range swarm.LocalChunks() {
		p.SendHave(i, remote, s)
	}
}
