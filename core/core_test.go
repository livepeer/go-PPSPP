package core

import "testing"

//  TestHandleDatagram tests the handleDatagram function
func TestHandleDatagram(t *testing.T) {
	// try a datagram on channel 1 with a HANDSHAKE
	msg := Msg{op: HANDSHAKE}
	d := Datagram{chanID: 1, msgs: []Msg{msg}}
	err := handleDatagram(d)
	if err == nil {
		t.Errorf("HANDSHAKES must use channel ID 0")
	}
}

// HANDSHAKE Tests TODO (from the RFC):
// The first datagram the initiating Peer P sends to Peer Q MUST start with a HANDSHAKE message
// Handshake message must contain:
// - a channelID, chanP, randomly chosen as specified in Section 12.1
// - the metadata of Swarm S, encoded as protocol options, as specified in Section 7. In particular, the initiating Peer P MUST include the swarm ID.
// - The capabilities of Peer P, in particular, its supported protocol versions, "Live Discard Window" (in case of a live swarm) and "Supported Messages", encoded as protocol options.
// This datagram MAY also contain some minor additional payload, e.g., HAVE messages to indicate Peer P's current progress, but it MUST NOT include any heavy payload (defined in Section 1.3), such as a DATA message.
