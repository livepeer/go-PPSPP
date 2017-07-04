package core

import (
	"flag"
	"testing"
)

func TestSendRequest(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up the peer
	remote := StringPeerID{"p2"}
	sid := SwarmID(42)
	remoteCID := ChanID(34)
	p := setupPeerWithHandshake(t, remote, remoteCID, sid, 1)
	n := p.n.(*StubNetwork)

	// Call SendRequest
	start := ChunkID(14)
	end := ChunkID(23)
	err := p.P.SendRequest(start, end, remote, sid)
	if err != nil {
		t.Fatal(err)
	}

	// Check the sent datagram for errors
	if num := n.NumSentDatagrams(); num != 1 {
		t.Fatalf("sent %d datagrams", num)
	}
	d := n.ReadSentDatagram()
	if c := d.ChanID; c != remoteCID {
		t.Fatalf("request should be on channel %d, got %d", remoteCID, c)
	}
	if num := len(d.Msgs); num != 1 {
		t.Fatalf("datagram with request should have 1 msg, got %d", num)
	}
	m := d.Msgs[0]
	if op := m.Op; op != Request {
		t.Fatalf("expected request op, got %v", op)
	}
	req, ok := m.Data.(RequestMsg)
	if !ok {
		t.Fatalf("RequestMsg type assertion failed")
	}
	if gotStart := req.Start; gotStart != start {
		t.Errorf("expected start=%d, got %d", start, gotStart)
	}
	if gotEnd := req.End; gotEnd != end {
		t.Errorf("expected end=%d, got %d", end, gotEnd)
	}
}

func TestHandleRequest(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up the peer
	chunkSize := 16
	remote := StringPeerID{"p2"}
	sid := SwarmID(42)
	remoteCID := ChanID(34)
	p := setupPeerWithHandshake(t, remote, remoteCID, sid, chunkSize)
	n := p.n.(*StubNetwork)

	// Add the local chunks that will be requested
	start := ChunkID(94)
	end := ChunkID(96)
	data := map[ChunkID]string{0: "aaaaAAAAaaaaAAAA", 1: "bbbbBBBBbbbbBBBB", 2: "ccccCCCCccccCCCC"}
	for i, s := range data {
		b := []byte(s)
		if err := p.P.AddLocalChunk(sid, start+i, b); err != nil {
			t.Fatal(err)
		}
	}

	// Get the local channel ID for injecting the Request
	prot := p.P.(*ppspp)
	s, err := prot.Swarm(sid)
	if err != nil {
		t.Fatalf("swarm not found at sid=%d: %v", sid, err)
	}
	theirs, ok := s.ChanID(remote)
	if !ok {
		t.Fatalf("channel id not found at peer id %v", remote)
	}

	// Inject a Request message for a chunk range
	m, err := messagize(RequestMsg{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	d := datagramize(theirs, m)
	if err := n.InjectIncomingDatagram(d, remote); err != nil {
		t.Fatal(err)
	}

	// Check the sent datagram for errors
	if num := n.NumSentDatagrams(); num != 1 {
		t.Fatalf("sent %d datagrams", num)
	}
	dsent := n.ReadSentDatagram()
	checkSendDataDatagram(t, dsent, start, end, data, chunkSize, remoteCID)
}
