package main

import (
	"flag"
	"math/rand"
	"time"

	"fmt"

	"github.com/golang/glog"
	"github.com/livepeer/go-PPSPP/core"
)

func main() {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up two peers, exchange IDs/addrs, and connect them.
	swarmMetadata := core.SwarmMetadata{ID: core.SwarmID(7), ChunkSize: 16}
	p1dataCh := make(chan core.DataMsg)
	config1 := core.SwarmConfig{
		Metadata: swarmMetadata,
		DataHandler: func(msg core.DataMsg) {
			p1dataCh <- msg
		},
	}
	config2 := core.SwarmConfig{
		Metadata: swarmMetadata,
	}
	sid := swarmMetadata.ID
	p1, p2, err := setupTwoPeerSwarm(234, config1, config2)
	if err != nil {
		glog.Fatalf("error setting up peers: %v", err)
	}
	glog.Infof("Data exchange between %s and %s on swarm %v\n", p1.ID(), p2.ID(), sid)

	// p1: kick off the HANDSHAKE
	if err := p1.P.StartHandshake(p2.ID(), swarmMetadata.ID); err != nil {
		glog.Fatalf("error starting handshake: %v", err)
	}

	time.Sleep(3 * time.Second)

	// p2: Add the source data to a swarm object
	p2data := map[core.ChunkID]string{0: "aaaaAAAAaaaaAAAA", 1: "bbbbBBBBbbbbBBBB",
		2: "ccccCCCCccccCCCC", 3: "ddddDDDDddddDDDD", 4: "eeeeEEEEeeeeEEEE", 5: "ffffFFFFffffFFFF"}
	numChunks := len(p2data)
	swarm2, err := p2.P.Swarm(sid)
	if err != nil {
		glog.Fatalf("sendHaves could not find swarm %v: %v", sid, err)
	}
	for i, data := range p2data {
		c := core.Chunk{ID: i, B: []byte(data)}
		if err := swarm2.AddLocalChunk(i, &c); err != nil {
			glog.Fatalf("sendHaves error adding local chunk: %v", err)
		}
	}

	// p2: Send HAVE messages to p1 for all of the chunks in the source data
	start := core.ChunkID(0)
	end := core.ChunkID(numChunks - 1)
	if err := p2.P.SendHave(start, end, p1.ID(), sid); err != nil {
		glog.Fatalf("sendHaves error: %v", err)
	}

	// We expect to receive the chunks very quickly.
	timeout := time.After(3 * time.Second)
	var receivedChunks int
	for receivedChunks != len(p2data) {
		select {
		case msg := <-p1dataCh:
			receivedChunks += int(msg.End-msg.Start) + 1
		case <-timeout:
			glog.Fatal("failed to receive data chunks in time")
		}
	}

	// p1: Should have requested and received the chunks from p2.
	// Check that it is in p1's swarm object.
	swarm1, err := p1.P.Swarm(sid)
	if err != nil {
		glog.Error(err)
	}
	content, err := swarm1.DataFromLocalChunks(0, core.ChunkID(numChunks-1))
	if err != nil {
		glog.Fatalf("content error: %v", err)
	}
	for i := 0; i < numChunks; i++ {
		for j := 0; j < swarm1.ChunkSize(); j++ {
			bref := []byte(p2data[core.ChunkID(i)])[j]
			bcontent := content[(i*swarm1.ChunkSize())+j]
			if bref != bcontent {
				glog.Errorf("local content mismatch chunk %d byte %d, bref=0x%x, bcontent=0x%x", i, j, bref, bcontent)
			}
		}
	}
	glog.Infof("content=%v", content)

	// p1: Send closing handshake
	if err := p1.P.SendClosingHandshake(p2.ID(), sid); err != nil {
		glog.Fatalf("error sending closing handshake: %v", err)
	}

	// Wait for the closing handshake to happen
	time.Sleep(3 * time.Second)

	// Disconnect the peers
	p1.Disconnect(p2.ID())
	p2.Disconnect(p1.ID())
}

func setupTwoPeerSwarm(seed int64, config1 core.SwarmConfig, config2 core.SwarmConfig) (*core.Peer, *core.Peer, error) {
	rand.Seed(seed)
	port1 := rand.Intn(100) + 10000
	port2 := port1 + 1
	p1, err := core.NewLibp2pPeer(port1, core.NewPpspp())
	if err != nil {
		return nil, nil, err
	}
	p2, err := core.NewLibp2pPeer(port2, core.NewPpspp())
	if err != nil {
		return nil, nil, err
	}

	addrs1 := p1.Addrs()
	addrs2 := p2.Addrs()
	p1.AddAddrs(p2.ID(), addrs2)
	p2.AddAddrs(p1.ID(), addrs1)

	p1.P.AddSwarm(config1)
	p2.P.AddSwarm(config2)
	err1 := p1.Connect(p2.ID())
	if err1 != nil {
		return nil, nil, fmt.Errorf("%v could not connect to %v: %v", p1.ID(), p2.ID(), err1)
	}
	err2 := p2.Connect(p1.ID())
	if err2 != nil {
		return nil, nil, fmt.Errorf("%v could not connect to %v: %v", p2.ID(), p1.ID(), err2)
	}
	return p1, p2, nil
}
