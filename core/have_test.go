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
	if c := d.ChanID; c != remoteCID {
		t.Fatalf("have should be on channel %d, got %d", remoteCID, c)
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

func setupPeerWithHandshake(t *testing.T, remote PeerID, remoteCID ChanID, sid SwarmID, chunkSize int) *Peer {
	// Set up the peer
	p := newStubNetworkPeer("p1")
	swarmMetadata := SwarmMetadata{ID: sid, ChunkSize: chunkSize}
	p.P.AddSwarm(swarmMetadata)
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
