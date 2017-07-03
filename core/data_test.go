package core

import (
	"flag"
	"testing"
)

func TestSendData(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up the peer
	chunkSize := 16
	remote := StringPeerID{"p2"}
	sid := SwarmID(42)
	remoteCID := ChanID(34)
	p := setupPeerWithHandshake(t, remote, remoteCID, sid, chunkSize)
	n := p.n.(*StubNetwork)

	// Add the local chunks
	start := ChunkID(14)
	end := ChunkID(start + 4)
	data := map[ChunkID]string{0: "aaaaAAAAaaaaAAAA", 1: "bbbbBBBBbbbbBBBB",
		2: "ccccCCCCccccCCCC", 3: "ddddDDDDddddDDDD", 4: "eeeeEEEEeeeeEEEE"}
	for i, s := range data {
		b := []byte(s)
		if err := p.P.AddLocalChunk(sid, start+i, b); err != nil {
			t.Fatal(err)
		}
	}

	// Call SendData
	if err := p.P.SendData(start, end, remote, sid); err != nil {
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
	for i := 0; i < len(data); i++ {
		for j := 0; j < chunkSize; j++ {
			bref := []byte(data[ChunkID(i)])[j]
			bgot := dataMsg.Data[(i*chunkSize)+j]
			if bref != bgot {
				t.Errorf("data mismatch chunk %d byte %d, bref=0x%x, bgot=0x%x", i, j, bref, bgot)
			}
		}
	}
}
