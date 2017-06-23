package core_test

import (
	"flag"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/glog"
	ps "github.com/libp2p/go-libp2p-peerstore"

	"github.com/livepeer/go-PPSPP/core"
)

// TestNetworkHandshake tests a handshake between two peers on two different ports
func TestNetworkHandshake(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// This is the bootstrap part -- set up the peers, exchange IDs/addrs, and
	// connect them in one thread.
	sid := core.SwarmID(8)
	p1, p2 := setupTwoPeerSwarm(t, 666, sid)
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

	p1.Disconnect(p2.ID())
	p2.Disconnect(p1.ID())
}

func setupTwoPeerSwarm(t *testing.T, seed int64, sid core.SwarmID) (*core.Peer, *core.Peer) {
	rand.Seed(seed)
	port1 := rand.Intn(100) + 10000
	port2 := port1 + 1
	p1 := core.NewPeer(port1)
	p2 := core.NewPeer(port2)
	peerExchangeIDAddr(p1, p2)
	p1.AddSwarm(sid)
	p2.AddSwarm(sid)
	_, err1 := p1.Connect(p2.ID())
	if err1 != nil {
		t.Fatalf("%v could not connect to %v", p1.ID(), p2.ID())
	}
	_, err2 := p2.Connect(p1.ID())
	if err2 != nil {
		t.Fatalf("%v could not connect to %v", p2.ID(), p1.ID())
	}
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

	reference := map[core.ChunkID]string{0: "aaaaAAAAaaaaAAAA", 1: "bbbbBBBBbbbbBBBB",
		2: "ccccCCCCccccCCCC", 3: "ddddDDDDddddDDDD", 4: "eeeeEEEEeeeeEEEE", 5: "ffffFFFFffffFFFF"}

	// This is the bootstrap part -- set up the peers, exchange IDs/addrs, and
	// connect them in one thread.
	sid := core.SwarmID(7)
	p1, p2 := setupTwoPeerSwarm(t, 234, sid)
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

	swarm, err1 := p1.Swarm(sid)
	if err1 != nil {
		t.Fatal(err1)
	}
	content, err2 := swarm.DataFromLocalChunks(0, core.ChunkID(len(reference)-1))
	if err2 != nil {
		t.Errorf("content error: %v", err2)
	}
	for i := 0; i < len(reference); i++ {
		for j := 0; j < swarm.ChunkSize(); j++ {
			bref := []byte(reference[core.ChunkID(i)])[j]
			bcontent := content[(i*swarm.ChunkSize())+j]
			if bref != bcontent {
				t.Errorf("local content mismatch chunk %d byte %d, bref=0x%x, bcontent=0x%x", i, j, bref, bcontent)
			}
		}
	}
	glog.Infof("content=%v", content)

	// Second phase: close the handshake in one thread, wait in the other.
	// Each thread checks that the channel is gone before setting the done channel
	done3 := make(chan bool, 1)
	done4 := make(chan bool, 1)
	go waitCloseNetworkHandshake(t, p1, p2.ID(), sid, done3)
	go closeNetworkHandshake(t, p2, p1.ID(), sid, done4)
	<-done3
	<-done4

	p1.Disconnect(p2.ID())
	p2.Disconnect(p1.ID())
}

func sendHaves(t *testing.T, ref map[core.ChunkID]string, s core.SwarmID, p *core.Peer, remote core.PeerID) {
	swarm, err1 := p.Swarm(s)
	if err1 != nil {
		t.Fatalf("sendHaves could not find swarm %v: %v", s, err1)
	}
	for i, data := range ref {
		c := core.Chunk{ID: i, B: []byte(data)}
		swarm.AddLocalChunk(i, &c)
	}
	start := core.ChunkID(0)
	end := core.ChunkID(len(ref) - 1)
	err2 := p.SendHave(start, end, remote, s)
	if err2 != nil {
		t.Fatalf("sendHaves error: %v", err2)
	}
}

// func sendRequests(t *testing.T, numChunks int, s core.SwarmID, p *core.Peer, remote core.PeerID)
