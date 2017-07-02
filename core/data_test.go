package core

import (
	"flag"
	"testing"
)

func TestSendData(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up the peer
	remote := StringPeerID{"p2"}
	sid := SwarmID(42)
	remoteCID := ChanID(34)
	p := setupPeerWithHandshake(t, remote, remoteCID, sid)
	n := p.n.(*StubNetwork)

	t.Fatal("TODO: set up data to be sent")

	// Call SendData
	start := ChunkID(14)
	end := ChunkID(23)
	err := p.P.SendData(start, end, remote, sid)
	if err != nil {
		t.Fatal(err)
	}

	// Check the sent datagram for errors
	if num := n.NumSentDatagrams(); num != 1 {
		t.Fatalf("sent %d datagrams", num)
	}
	d := n.ReadSentDatagram()
	if c := d.ChanID; c != remoteCID {
		t.Fatalf("data should be on channel %d, got %d", remoteCID, c)
	}
	if num := len(d.Msgs); num != 1 {
		t.Fatalf("datagram with data should have 1 msg, got %d", num)
	}
	m := d.Msgs[0]
	if op := m.Op; op != Data {
		t.Fatalf("expected data op, got %v", op)
	}
	dataMsg, ok := m.Data.(DataMsg)
	if !ok {
		t.Fatalf("DataMsg type assertion failed")
	}
	if gotStart := dataMsg.Start; gotStart != start {
		t.Errorf("expected start=%d, got %d", start, gotStart)
	}
	if gotEnd := dataMsg.End; gotEnd != end {
		t.Errorf("expected end=%d, got %d", end, gotEnd)
	}
}
