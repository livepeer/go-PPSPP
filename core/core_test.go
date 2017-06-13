package core

import "testing"

func TestGoodHandshake(t *testing.T) {
	p := NewPeer()

	// start the handshake and check that the peer's channel moves to WAIT_HANDSHAKE
	ours := p.startHandshake()
	c := p.chans[ours]
	if c.state != WAIT_HANDSHAKE {
		t.Errorf("bad state %v", c.state)
	}

	// inject a handshake reply and check that the peer's channel moves to READY
	h := Handshake{14}
	m := Msg{op: HANDSHAKE, data: h}
	d := Datagram{chanID: ours, msgs: []Msg{m}}
	err := p.HandleDatagram(d)
	if err != nil {
		t.Errorf("HandleDatagram error: %v", err)
	}
	if c.state != READY {
		t.Errorf("peer not ready after HANDSHAKE reply, state=%v", c.state)
	}
}

func TestBadHandshake(t *testing.T) {
	p := NewPeer()

	// start the handshake and check that the peer's channel moves to WAIT_HANDSHAKE
	ours := p.startHandshake()
	c := p.chans[ours]
	if c.state != WAIT_HANDSHAKE {
		t.Errorf("bad state %v", c.state)
	}

	// inject a bad handshake reply and check that we notice it
	h := Handshake{0}
	m := Msg{op: HANDSHAKE, data: h}
	d := Datagram{chanID: ours, msgs: []Msg{m}}
	err := p.HandleDatagram(d)
	if err == nil {
		t.Errorf("HandleDatagram did not catch bad handshake reply")
	}
}

// HANDSHAKE Tests TODO (from the RFC):
// The first datagram the initiating Peer P sends to Peer Q MUST start with a HANDSHAKE message
// Handshake message must contain:
// - a channelID, chanP, randomly chosen as specified in Section 12.1
// - the metadata of Swarm S, encoded as protocol options, as specified in Section 7. In particular, the initiating Peer P MUST include the swarm ID.
// - The capabilities of Peer P, in particular, its supported protocol versions, "Live Discard Window" (in case of a live swarm) and "Supported Messages", encoded as protocol options.
// This datagram MAY also contain some minor additional payload, e.g., HAVE messages to indicate Peer P's current progress, but it MUST NOT include any heavy payload (defined in Section 1.3), such as a DATA message.
