package core

import (
	"bytes"
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
	checkSendDataDatagram(t, d, start, end, data, chunkSize, remoteCID)
}

func TestHandleData(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	// Set up the peer
	chunkSize := 16
	remote := StringPeerID{"p2"}
	sid := SwarmID(42)
	remoteCID := ChanID(34)
	p := setupPeerWithHandshake(t, remote, remoteCID, sid, chunkSize)
	n := p.n.(*StubNetwork)

	// Get the local channel ID for injecting the Request
	theirs := theirs(t, p, sid, remote)

	// Inject DataMsg
	start := ChunkID(14)
	end := ChunkID(start + 1)
	data := []byte("aaaaAAAAaaaaAAAAbbbbBBBBbbbbBBBB")
	m, err := messagize(DataMsg{Start: start, End: end, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	d := datagramize(theirs, m)
	if err := n.InjectIncomingDatagram(d, remote); err != nil {
		t.Fatal(err)
	}

	// Check that the data is stored locally
	swarm, err := p.P.Swarm(sid)
	if err != nil {
		t.Fatal(err)
	}
	gotData, err := swarm.DataFromLocalChunks(start, end)
	if err != nil {
		t.Fatal(err)
	}
	if cmp := bytes.Compare(data, gotData); cmp != 0 {
		t.Errorf("data mismatch %d, data=%v, gotData=%v", cmp, data, gotData)
	}

	// Check sent have message for errors
	if num := n.NumSentDatagrams(); num != 1 {
		t.Fatalf("sent %d datagrams", num)
	}
	dsent := n.ReadSentDatagram()
	checkSendHaveDatagram(t, dsent, start, end, remoteCID)
}

// checkSendDataDatagram checks the datagram for errors
// It should be a datagram with 1 message: a data message with the given parameters
func checkSendDataDatagram(t *testing.T, d *Datagram, start ChunkID, end ChunkID, chunks map[ChunkID]string, chunkSize int, remote ChanID) {
	if c := d.ChanID; c != remote {
		t.Fatalf("data should be on channel %d, got %d", remote, c)
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
	checkDataMsg(t, &dataMsg, start, end, chunks, chunkSize)
}

func checkDataMsg(t *testing.T, m *DataMsg, start ChunkID, end ChunkID, chunks map[ChunkID]string, chunkSize int) {
	if gotStart := m.Start; gotStart != start {
		t.Errorf("expected start=%d, got %d", start, gotStart)
	}
	if gotEnd := m.End; gotEnd != end {
		t.Errorf("expected end=%d, got %d", end, gotEnd)
	}
	for i := 0; i < len(chunks); i++ {
		for j := 0; j < chunkSize; j++ {
			bref := []byte(chunks[ChunkID(i)])[j]
			bgot := m.Data[(i*chunkSize)+j]
			if bref != bgot {
				t.Errorf("data mismatch chunk %d byte %d, bref=0x%x, bgot=0x%x", i, j, bref, bgot)
			}
		}
	}
}
