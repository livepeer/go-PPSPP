package core

import (
	"flag"
	"testing"
)

func TestStartHandshake(t *testing.T) {
	flag.Lookup("logtostderr").Value.Set("true")

	sid := SwarmID(7)

	p := newStubNetworkPeer("p1")

	p.P.StartHandshake(StringPeerID{"p2"}, sid)

	n := p.n.(*StubNetwork)

	if num := len(n.SentDatagrams); num != 1 {
		t.Fatalf("sent %d datagrams", num)
	}

	d := n.SentDatagrams[0]

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

func newStubNetworkPeer(id string) *Peer {
	p := newPpspp()

	n := NewStubNetwork(id)

	return NewPeer(n, p)
}
