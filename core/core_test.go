package core

import (
	"context"
	"log"
	"math/rand"
	"testing"

	"fmt"

	inet "github.com/libp2p/go-libp2p-net"
	ps "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
)

func TestNetworkHandshake(t *testing.T) {
	// Choose random ports between 10000-10100
	rand.Seed(666)
	port1 := rand.Intn(100) + 10000
	port2 := port1 + 1

	// Make 2 peers
	p1 := NewPeer(port1)
	p2 := NewPeer(port2)
	h1 := p1.h
	h2 := p2.h
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), ps.PermanentAddrTTL)
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), ps.PermanentAddrTTL)

	log.Printf("This is a conversation between %s and %s\n", h1.ID(), h2.ID())

	// Define a stream handler for host number 2
	const proto = "/example/1.0.0"
	p2.WrapSetStreamHandler(proto, t)

	// Create new stream from h1 to h2 and start the conversation
	stream, err := h1.NewStream(context.Background(), h2.ID(), proto)
	if err != nil {
		log.Fatal(err)
	}
	wrappedStream := WrapStream(stream)

	p1.startHandshake(wrappedStream)

	err2 := p1.HandleStream(wrappedStream)
	if err2 != nil {
		fmt.Println("error")
		t.Error(err2)
	}

	stream.Close()

	// TODO: check that both peers are in READY state
}

func (p *Peer) WrapSetStreamHandler(proto protocol.ID, t *testing.T) {
	fmt.Println("setting stream handler")
	p.h.SetStreamHandler(proto, func(stream inet.Stream) {
		log.Printf("%s: Received a stream", p.h.ID())
		wrappedStream := WrapStream(stream)
		defer stream.Close()
		err := p.HandleStream(wrappedStream)
		fmt.Println("handled stream")
		if err != nil {
			t.Fatal(err)
		}
	})
}

// HANDSHAKE Tests TODO (from the RFC):
// The first datagram the initiating Peer P sends to Peer Q MUST start with a HANDSHAKE message
// Handshake message must contain:
// - a channelID, chanP, randomly chosen as specified in Section 12.1
// - the metadata of Swarm S, encoded as protocol options, as specified in Section 7. In particular, the initiating Peer P MUST include the swarm ID.
// - The capabilities of Peer P, in particular, its supported protocol versions, "Live Discard Window" (in case of a live swarm) and "Supported Messages", encoded as protocol options.
// This datagram MAY also contain some minor additional payload, e.g., HAVE messages to indicate Peer P's current progress, but it MUST NOT include any heavy payload (defined in Section 1.3), such as a DATA message.
