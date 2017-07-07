package core

import (
	"flag"
	"testing"
)

func TestSendHave(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up the peer
	remote := StringPeerID{"p2"}
	sid := SwarmID(42)
	remoteCID := ChanID(34)
	p := setupPeerWithHandshake(t, remote, remoteCID, sid, 2)
	n := p.n.(*StubNetwork)

	// Call SendHave
	start := ChunkID(14)
	end := ChunkID(23)
	err := p.P.SendHave(start, end, remote, sid)
	if err != nil {
		t.Fatal(err)
	}

	// Check the sent datagram for errors
	if num := n.NumSentDatagrams(); num != 1 {
		t.Fatalf("sent %d datagrams", num)
	}
	d := n.ReadSentDatagram()
	checkSendHaveDatagram(t, d, start, end, remoteCID)
}

func TestHandleHave(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up the peer
	remote := StringPeerID{"p2"}
	sid := SwarmID(42)
	remoteCID := ChanID(34)
	p := setupPeerWithHandshake(t, remote, remoteCID, sid, 2)
	n := p.n.(*StubNetwork)

	// Get the local channel ID for injecting the Have
	prot := p.P.(*Ppspp)
	s, err := prot.Swarm(sid)
	if err != nil {
		t.Fatalf("swarm not found at sid=%d: %v", sid, err)
	}
	theirs, ok := s.ChanID(remote)
	if !ok {
		t.Fatalf("channel id not found at peer id %v", remote)
	}

	// Inject a Have message for a chunk range
	start := ChunkID(11)
	end := ChunkID(13)
	m, err := messagize(HaveMsg{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	d := datagramize(theirs, m)
	if err := n.InjectIncomingDatagram(d, remote); err != nil {
		t.Fatal(err)
	}

	// Check the sent datagram -- it should have a request for the chunk range
	if num := n.NumSentDatagrams(); num != 1 {
		t.Fatalf("sent %d datagrams", num)
	}
	dsent := n.ReadSentDatagram()
	if c := dsent.ChanID; c != remoteCID {
		t.Fatalf("request handshake should be on channel %d, got %d", remoteCID, c)
	}
	if num := len(dsent.Msgs); num == 0 {
		t.Fatal("datagram with reply handshake should have at least 1 msg")
	}
	msent := dsent.Msgs[0]
	if op := msent.Op; op != Request {
		t.Fatalf("expected request op, got %v", op)
	}
	r, ok := msent.Data.(RequestMsg)
	if !ok {
		t.Fatalf("RequestMsg type assertion failed")
	}
	if r.Start != start {
		t.Errorf("request start=%d, should be %d", r.Start, start)
	}
	if r.End != end {
		t.Errorf("request end=%d, should be %d", r.End, end)
	}
}

func setupPeerWithHandshake(t *testing.T, remote PeerID, remoteCID ChanID, sid SwarmID, chunkSize int) *Peer {
	// Set up the peer
	p := newStubNetworkPeer("p1")
	swarmMetadata := SwarmMetadata{ID: sid, ChunkSize: chunkSize}
	if err := p.P.AddSwarm(swarmMetadata); err != nil {
		t.Fatalf("setupPeerWithHandshake could not add swarm: %v", err)
	}
	n := p.n.(*StubNetwork)

	// Call StartHandshake
	p.P.StartHandshake(remote, sid)

	// Read the channel and swarm IDs from the sent handshake so we can mock up a reply
	d := n.ReadSentDatagram()
	m := d.Msgs[0]
	h, ok := m.Data.(HandshakeMsg)
	if !ok {
		t.Fatalf("handshake type assertion failed")
	}

	// Inject a reply handshake
	replyH := HandshakeMsg{C: remoteCID, S: sid}
	m = Msg{Op: Handshake, Data: replyH}
	msgs := []Msg{m}
	d = &Datagram{ChanID: h.C, Msgs: msgs}
	err := n.InjectIncomingDatagram(d, remote)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

// checkSendHaveDatagram checks the datagram for errors
// It should be a datagram with 1 message: a have message with the given parameters
func checkSendHaveDatagram(t *testing.T, d *Datagram, start ChunkID, end ChunkID, remote ChanID) {
	if c := d.ChanID; c != remote {
		t.Fatalf("have should be on channel %d, got %d", remote, c)
	}
	if num := len(d.Msgs); num != 1 {
		t.Fatalf("datagram with have should have 1 msg, got %d", num)
	}
	m := d.Msgs[0]
	if op := m.Op; op != Have {
		t.Fatalf("expected have op, got %v", op)
	}
	have, ok := m.Data.(HaveMsg)
	if !ok {
		t.Fatalf("HaveMsg type assertion failed")
	}
	if gotStart := have.Start; gotStart != start {
		t.Errorf("expected start=%d, got %d", start, gotStart)
	}
	if gotEnd := have.End; gotEnd != end {
		t.Errorf("expected end=%d, got %d", end, gotEnd)
	}
}
