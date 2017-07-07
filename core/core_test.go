package core_test

import (
	"flag"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/glog"

	"fmt"

	"github.com/livepeer/go-PPSPP/core"
)

// TestNetworkHandshake tests a handshake between two peers on two different ports
func TestNetworkHandshake(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// This is the bootstrap part -- set up the peers, exchange IDs/addrs, and
	// connect them in one thread.
	swarmMetadata := core.SwarmMetadata{ID: core.SwarmID(8), ChunkSize: 8}
	sid := swarmMetadata.ID
	p1, p2 := setupTwoPeerSwarm(t, 666, swarmMetadata)
	glog.Infof("Handshake between %s and %s on swarm %v\n", p1.ID(), p2.ID(), sid)

	// First phase: start handshake in one thread, wait in the other
	// Each thread checks that state is moved to ready before setting the done channel
	//done1 := make(chan bool, 1)
	errc1 := make(chan error, 1)
	errc2 := make(chan error, 1)
	go startNetworkHandshake(p1, p2.ID(), swarmMetadata, errc1)
	go waitNetworkHandshake(t, p2, p1.ID(), sid, errc2)
	if err := <-errc1; err != nil {
		t.Fatal(err)
	}
	if err := <-errc2; err != nil {
		t.Fatal(err)
	}

	// Second phase: close the handshake in one thread, wait in the other.
	// Each thread checks that the channel is gone before setting the done channel
	errc3 := make(chan error, 1)
	errc4 := make(chan error, 1)
	go waitCloseNetworkHandshake(p1, p2.ID(), sid, errc3)
	go closeNetworkHandshake(p2, p1.ID(), sid, errc4)
	if err := <-errc3; err != nil {
		t.Fatal(err)
	}
	if err := <-errc4; err != nil {
		t.Fatal(err)
	}

	p1.Disconnect(p2.ID())
	p2.Disconnect(p1.ID())

	// Test Disconnect by making sure another handshake fails
	err := p1.P.StartHandshake(p2.ID(), swarmMetadata.ID)
	if err == nil {
		t.Error("StartHandshake should fail after Disconnect")
	} else {
		glog.Infof("StartHandshake error as expected: %v", err)
	}
}

func setupTwoPeerSwarm(t *testing.T, seed int64, metadata core.SwarmMetadata) (*core.Peer, *core.Peer) {
	rand.Seed(seed)
	port1 := rand.Intn(100) + 10000
	port2 := port1 + 1
	p1, err := core.NewLibp2pPeer(port1, core.NewPpspp())
	if err != nil {
		t.Fatal(err)
	}
	p2, err := core.NewLibp2pPeer(port2, core.NewPpspp())
	if err != nil {
		t.Fatal(err)
	}
	peerExchangeIDAddr(p1, p2)
	if err := p1.P.AddSwarm(metadata); err != nil {
		t.Fatal(err)
	}
	if err := p2.P.AddSwarm(metadata); err != nil {
		t.Fatal(err)
	}
	err1 := p1.Connect(p2.ID())
	if err1 != nil {
		t.Fatalf("%v could not connect to %v: %v", p1.ID(), p2.ID(), err1)
	}
	err2 := p2.Connect(p1.ID())
	if err2 != nil {
		t.Fatalf("%v could not connect to %v: %v", p2.ID(), p1.ID(), err2)
	}
	return p1, p2
}

func startNetworkHandshake(p *core.Peer, remote core.PeerID, swarmMetadata core.SwarmMetadata, errc chan error) {
	// kick off the handshake
	if err := p.P.StartHandshake(remote, swarmMetadata.ID); err != nil {
		errc <- err
		return
	}

	// Sleep to give the other peer time to reply
	time.Sleep(3 * time.Second)

	// Check that this peer is Ready (means the handshake succeeded)
	errc <- checkState(swarmMetadata.ID, p, remote, core.Ready)
}

func waitNetworkHandshake(t *testing.T, p *core.Peer, remote core.PeerID, sid core.SwarmID, errc chan error) {
	time.Sleep(3 * time.Second)
	errc <- checkState(sid, p, remote, core.Ready)
}

func closeNetworkHandshake(p *core.Peer, remote core.PeerID, sid core.SwarmID, errc chan error) {
	if err := p.P.SendClosingHandshake(remote, sid); err != nil {
		errc <- err
		return
	}

	time.Sleep(3 * time.Second)

	errc <- checkNoChannel(sid, p, remote)
}

func waitCloseNetworkHandshake(p *core.Peer, remote core.PeerID, sid core.SwarmID, errc chan error) {
	time.Sleep(3 * time.Second)

	errc <- checkNoChannel(sid, p, remote)
}

// magic exchange of peer IDs and addrs
func peerExchangeIDAddr(p1 *core.Peer, p2 *core.Peer) {
	addrs1 := p1.Addrs()
	addrs2 := p2.Addrs()
	p1.AddAddrs(p2.ID(), addrs2)
	p2.AddAddrs(p1.ID(), addrs1)
}

// checkState checks that the peer's ProtocolState is equal to state for swarm sid for the remote peer
func checkState(sid core.SwarmID, p *core.Peer, remote core.PeerID, state core.ProtocolState) error {
	foundState, err := p.P.ProtocolState(sid, remote)
	if err != nil {
		return fmt.Errorf("could not get state for %v: %v", p.ID(), err)
	}
	if foundState != state {
		return fmt.Errorf("%v state=%v, not %v after handshake", p.ID(), foundState, state)
	}
	return nil
}

// checkNoChannel checks that peer p does not have a channel for swarm sid for the remote peer
func checkNoChannel(sid core.SwarmID, p *core.Peer, remote core.PeerID) error {
	// This is a bit of a hacky round-about way to check that there is not channel
	// We should write a function with receiver (p *Peer) that actually checks if the channel exists
	// in the Peer object, but the ProtocolState function essentially does that already.
	foundState, err := p.P.ProtocolState(sid, remote)
	if !(foundState == core.Unknown && err != nil) {
		return fmt.Errorf("%v found a channel for sid=%v, remote=%v", p.ID(), sid, remote)
	}
	return nil
}

// Test NetworkDataExchange tests data exchange between two peers over two different ports
func TestNetworkDataExchange(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	reference := map[core.ChunkID]string{0: "aaaaAAAAaaaaAAAA", 1: "bbbbBBBBbbbbBBBB",
		2: "ccccCCCCccccCCCC", 3: "ddddDDDDddddDDDD", 4: "eeeeEEEEeeeeEEEE", 5: "ffffFFFFffffFFFF"}

	// This is the bootstrap part -- set up the peers, exchange IDs/addrs, and
	// connect them in one thread.
	swarmMetadata := core.SwarmMetadata{ID: core.SwarmID(7), ChunkSize: 16}
	sid := swarmMetadata.ID
	p1, p2 := setupTwoPeerSwarm(t, 234, swarmMetadata)
	glog.Infof("Data exchange between %s and %s on swarm %v\n", p1.ID(), p2.ID(), sid)

	// First phase: start handshake in one thread, wait in the other
	// Each thread checks that state is moved to ready before setting the done channel
	errc1 := make(chan error, 1)
	errc2 := make(chan error, 1)
	go startNetworkHandshake(p1, p2.ID(), swarmMetadata, errc1)
	go waitNetworkHandshake(t, p2, p1.ID(), sid, errc2)
	<-errc1
	<-errc2

	if err := sendHaves(reference, sid, p2, p1.ID()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(3 * time.Second)

	swarm, err1 := p1.P.Swarm(sid)
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
	errc3 := make(chan error, 1)
	errc4 := make(chan error, 1)
	go waitCloseNetworkHandshake(p1, p2.ID(), sid, errc3)
	go closeNetworkHandshake(p2, p1.ID(), sid, errc4)
	if err := <-errc3; err != nil {
		t.Fatal(err)
	}
	if err := <-errc4; err != nil {
		t.Fatal(err)
	}

	p1.Disconnect(p2.ID())
	p2.Disconnect(p1.ID())
}

func sendHaves(ref map[core.ChunkID]string, s core.SwarmID, p *core.Peer, remote core.PeerID) error {
	swarm, err1 := p.P.Swarm(s)
	if err1 != nil {
		return fmt.Errorf("sendHaves could not find swarm %v: %v", s, err1)
	}
	for i, data := range ref {
		c := core.Chunk{ID: i, B: []byte(data)}
		if err := swarm.AddLocalChunk(i, &c); err != nil {
			return fmt.Errorf("sendHaves error adding local chunk: %v", err)
		}
	}
	start := core.ChunkID(0)
	end := core.ChunkID(len(ref) - 1)
	err2 := p.P.SendHave(start, end, remote, s)
	if err2 != nil {
		return fmt.Errorf("sendHaves error: %v", err2)
	}
	return nil
}
