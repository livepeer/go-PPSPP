package core

import (
	"flag"
	"testing"
)

func TestStartHandshake(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up the peer
	sid := SwarmID(7)
	p := newStubNetworkPeer("p1")
	n := p.n.(*StubNetwork)

	// Call StartHandshake
	p.P.StartHandshake(StringPeerID{"p2"}, sid, nil)

	// Check the sent handshake for errors
	if num := n.NumSentDatagrams(); num != 1 {
		t.Fatalf("sent %d datagrams", num)
	}
	d := n.ReadSentDatagram()
	if c := d.ChanID; c != 0 {
		t.Fatalf("start handshake should be on channel 0, got %d", c)
	}
	if num := len(d.Msgs); num != 1 {
		t.Fatalf("start handshake should have 1 msg, got %d", num)
	}
	m := d.Msgs[0]
	if op := m.Op; op != Handshake {
		t.Fatalf("expected handshake op, got %v", op)
	}
	h, ok := m.Data.(HandshakeMsg)
	if !ok {
		t.Fatalf("handshake type assertion failed")
	}
	if c := h.C; c == 0 {
		t.Error("handshake cannot have C=0 (should be random positive int)")
	}
	if s := h.S; s != sid {
		t.Errorf("handshake sid should be %d, got %d", sid, s)
	}
}

func TestHandleHandshake(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up the peer
	sid := SwarmID(16)
	p := newStubNetworkPeer("p1")
	remote := StringPeerID{"p2"}
	stubNetwork, ok := p.n.(*StubNetwork)
	if !ok {
		t.Fatal("StubNetwork type assertion failed")
	}
	cid := ChanID(4)
	swarmMetadata := SwarmMetadata{ID: sid, ChunkSize: 8}
	p.P.AddSwarm(swarmMetadata)

	// Inject a starting handshake
	h := HandshakeMsg{C: cid, S: sid}
	m := Msg{Op: Handshake, Data: h}
	msgs := []Msg{m}
	d := &Datagram{ChanID: 0, Msgs: msgs}
	err := stubNetwork.InjectIncomingDatagram(d, remote)
	if err != nil {
		t.Fatal(err)
	}

	// Check the reply handshake for errors
	if num := stubNetwork.NumSentDatagrams(); num != 1 {
		t.Fatalf("sent %d datagrams", num)
	}
	dsent := stubNetwork.ReadSentDatagram()
	if c := dsent.ChanID; c != cid {
		t.Fatalf("reply handshake should be on channel %d, got %d", cid, c)
	}
	if num := len(dsent.Msgs); num == 0 {
		t.Fatal("datagram with reply handshake should have at least 1 msg")
	}
	msent := dsent.Msgs[0]
	if op := msent.Op; op != Handshake {
		t.Fatalf("expected handshake op, got %v", op)
	}
	replyH, ok2 := msent.Data.(HandshakeMsg)
	if !ok2 {
		t.Fatalf("HandshakeMsg type assertion failed")
	}
	replyC := replyH.C
	if replyC == 0 {
		t.Error("handshake cannot have C=0 (should be random positive int)")
	}
	if s := replyH.S; s != sid {
		t.Errorf("handshake sid should be %d, got %d", sid, s)
	}

	// Inject a closing handshake
	h = HandshakeMsg{C: 0, S: sid}
	m = Msg{Op: Handshake, Data: h}
	msgs = []Msg{m}
	d = &Datagram{ChanID: replyC, Msgs: msgs}
	err = stubNetwork.InjectIncomingDatagram(d, remote)
	if err != nil {
		t.Fatal(err)
	}

	// There should be no reply
	if num := stubNetwork.NumSentDatagrams(); num != 0 {
		t.Fatalf("sent %d datagrams", num)
	}
}

// TestHandshakeRace tests the case where two peers start handshakes to each other at the same time
func TestHandshakeRace(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up p1
	p := newStubNetworkPeer("p1")
	remote := StringPeerID{"p2"}
	stubNetwork, ok := p.n.(*StubNetwork)
	if !ok {
		t.Fatal("StubNetwork type assertion failed")
	}
	cid := ChanID(4)

	// Set up a swarm
	sid := SwarmID(8)
	swarmMetadata := SwarmMetadata{ID: sid, ChunkSize: 8}
	p.P.AddSwarm(swarmMetadata)

	pReady := make(chan int, 1)
	onPReady := func(id PeerID) {
		pReady <- 1
	}
	// Start handshake with p2
	p.P.StartHandshake(StringPeerID{"p2"}, swarmMetadata.ID, onPReady)

	// Inject a valid starting handshake from p2
	h := HandshakeMsg{C: cid, S: swarmMetadata.ID}
	m := Msg{Op: Handshake, Data: h}
	msgs := []Msg{m}
	d := &Datagram{ChanID: 0, Msgs: msgs}
	err := stubNetwork.InjectIncomingDatagram(d, remote)
	if err != nil {
		t.Fatal(err)
	}

	// p1 should interpret the starting handshake from p2 as a reply, so it
	// should now be in the Ready state
	ok, err = checkState(sid, p, remote, Ready)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("p1 not in ready state after handshake")
	}
	<-pReady
}
