package core

import (
	"bufio"
	"context"
	"errors"
	"fmt"

	json0 "encoding/json"

	"encoding/gob"

	"bytes"

	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	ps "github.com/libp2p/go-libp2p-peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
	multicodec "github.com/multiformats/go-multicodec"
	json "github.com/multiformats/go-multicodec/json"
)

// MsgError is an error that happens while handling an incoming message
type MsgError struct {
	c    ChanID
	m    Msg
	info string
}

func (e MsgError) Error() string {
	return fmt.Sprintf("message error on channel %v: %v", e.c, e.info)
}

// ChanID identifies a channel
type ChanID uint

// Opcode identifies the type of message
type Opcode uint8

const (
	handshake Opcode = 13 // weird number so it's easier to notice in debug info
)

// MsgData holds the data payload of a message
type MsgData interface{}

// Handshake holds a handshake message data payload
type Handshake struct {
	C ChanID
	// TODO: swarm SwarmMetadata
	// TODO: peer capabilities
}

// Msg holds a protocol message
type Msg struct {
	Op   Opcode
	Data MsgData
}

// msgAux is an auxiliary struct that looks like Msg except it has
// a []byte to store the incoming gob for MsgData
// (see marshal/unmarshal functions on Msg)
type msgAux struct {
	Op   Opcode
	Data []byte
}

// UnmarshalJSON handles the deserializing of a message.
//
// We can't get away with off-the-shelf JSON, because
// we're using an interface type for MsgData, which causes problems
// on the decode side.
func (m *Msg) UnmarshalJSON(b []byte) error {
	// Use builtin json to unmarshall into aux
	var aux msgAux
	json0.Unmarshal(b, &aux)

	// The Op field in aux is already what we want for m.Op
	m.Op = aux.Op

	// decode the gob in aux.Data and put it in m.Data
	dec := gob.NewDecoder(bytes.NewBuffer(aux.Data))
	switch aux.Op {
	case handshake:
		var h Handshake
		err := dec.Decode(&h)
		if err != nil {
			return errors.New("failed to decode handshake")
		}
		m.Data = h
	default:
		return errors.New("failed to decode message data")
	}

	return nil
}

// MarshalJSON handles the serializing of a message.
//
// See note above UnmarshalJSON for the reason for the custom MarshalJSON
func (m Msg) MarshalJSON() ([]byte, error) {
	// Encode m.Data into a gob
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	switch m.Data.(type) {
	case Handshake:
		gob.Register(Handshake{})
		err := enc.Encode(m.Data.(Handshake))
		if err != nil {
			return nil, errors.New("Failed to marshal Handshake")
		}
	default:
		return nil, errors.New("failed to marshal message data")
	}

	// build an aux and marshal using built-in json
	aux := msgAux{Op: m.Op, Data: b.Bytes()}
	return json0.Marshal(aux)
}

// Datagram holds a protocol datagram
type Datagram struct {
	ChanID ChanID
	Msgs   []Msg
}

// ProtocolState is a per-channel state local to a peer
type ProtocolState uint

const (
	begin         ProtocolState = 0
	waitHandshake ProtocolState = 1 // waiting for ack of first handshake
	ready         ProtocolState = 2
)

// Chan holds the current state of a channel
type Chan struct {
	ours   ChanID // receiving channel id (unique)
	theirs ChanID // sending channel id
	state  ProtocolState
}

// Peer is currently just a couple of things related to a peer (as defined in the RFC)
type Peer struct {
	chans map[ChanID](*Chan) // indexed by our local channel ID
	h     host.Host
}

// NewPeer makes and initializes a new peer
func NewPeer(port int) *Peer {
	chans := make(map[ChanID](*Chan))
	chans[0] = &Chan{0, 0, begin}
	h := NewBasicHost(port)
	return &Peer{chans: chans, h: h}
}

// HandleStream handles an incoming stream
// TODO: not sure how this works wrt multiple incoming datagrams
func (p *Peer) HandleStream(ws *WrappedStream) error {
	fmt.Println("handling stream")
	d, err := receiveDatagram(ws)
	fmt.Println("recvd Datagram")
	if err != nil {
		return err
	}
	return p.handleDatagram(d, ws)
}

// receiveDatagram reads and decodes a datagram from the stream
func receiveDatagram(ws *WrappedStream) (*Datagram, error) {
	var d Datagram
	err := ws.dec.Decode(&d)
	fmt.Printf("receiving datagram %v\n", d)
	if err != nil {
		return nil, err
	}
	h, ok2 := d.Msgs[0].Data.(Handshake)
	if !ok2 {
		return nil, MsgError{info: "could not convert 2"}
	}
	fmt.Printf("decoded handshake=%v\n", h)
	return &d, nil
}

// sendDatagram encodes and writes a datagram to the stream
func sendDatagram(d Datagram, ws *WrappedStream) error {
	fmt.Printf("sending datagram %v\n", d)
	err := ws.enc.Encode(d)
	// Because output is buffered with bufio, we need to flush!
	ws.w.Flush()
	return err
}

func (p *Peer) handleDatagram(d *Datagram, ws *WrappedStream) error {
	fmt.Printf("handling datagram %v\n", d)
	if len(d.Msgs) == 0 {
		return errors.New("no messages in datagram")
	}
	for _, msg := range d.Msgs {
		err := p.handleMsg(p.chans[d.ChanID], msg, ws)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Peer) handleMsg(c *Chan, m Msg, ws *WrappedStream) error {
	switch m.Op {
	case handshake:
		return p.handleHandshake(c, m, ws)
	default:
		return MsgError{c: c.ours, m: m, info: "bad opcode"}
	}
}

func (p *Peer) handleHandshake(c *Chan, m Msg, ws *WrappedStream) error {
	fmt.Println("handling handshake")
	h, ok := m.Data.(Handshake)
	if !ok {
		return MsgError{c: c.ours, m: m, info: "could not convert to HANDSHAKE"}
	}
	switch c.state {
	case begin:
		fmt.Println("in begin state")
		if c.ours != 0 {
			return MsgError{c: c.ours, m: m, info: "HANDSHAKE must use channel ID 0"}
		}
		if h.C < 1 {
			return MsgError{c: c.ours, m: m, info: "HANDSHAKE cannot request channel ID 0"}
		}
		ours := chooseOurID()
		p.setupChan(ours, h.C, ready)
		p.sendReplyHandshake(ours, h.C, ws)
	case waitHandshake:
		fmt.Println("in waitHandshake state")
		if h.C == 0 {
			fmt.Println("received closing handshake")
			p.closeChannel(c.ours)
		} else {
			c.theirs = h.C
			c.state = ready
		}
	case ready:
		fmt.Println("in ready state")
		if h.C == 0 {
			fmt.Println("received closing handshake")
			p.closeChannel(c.ours)
		} else {
			return MsgError{c: c.ours, m: m, info: "got non-closing handshake while in ready state"}
		}
	default:
		return MsgError{c: c.ours, m: m, info: "bad channel state"}
	}
	return nil
}

func (p *Peer) closeChannel(c ChanID) {
	fmt.Println("closing channel")
	delete(p.chans, c)
}

func (p *Peer) setupChan(ours ChanID, theirs ChanID, state ProtocolState) error {
	if ours < 1 {
		return errors.New("cannot setup channel with ours<1")
	}
	p.chans[ours] = &Chan{ours: ours, theirs: theirs, state: state}
	return nil
}

func (p *Peer) startHandshake(ws *WrappedStream) ChanID {
	fmt.Println("starting handshake")
	ours := chooseOurID()
	// their channel is 0 until they reply with a handshake
	p.setupChan(ours, 0, begin)
	p.chans[ours].state = waitHandshake
	p.sendReqHandshake(ours, ws)
	return ours
}

func (p *Peer) sendReqHandshake(ours ChanID, ws *WrappedStream) {
	fmt.Println("Peer sending request HANDSHAKE")
	glog.Info("Peer sending request HANDSHAKE")
	p.sendHandshake(ours, 0, ws)
}

func (p *Peer) sendReplyHandshake(ours ChanID, theirs ChanID, ws *WrappedStream) {
	fmt.Println("Peer sending reply HANDSHAKE")
	glog.Info("Peer sending reply HANDSHAKE")
	p.sendHandshake(ours, theirs, ws)
}

func (p *Peer) sendClosingHandshake(ours ChanID, ws *WrappedStream) {
	fmt.Println("Peer sending closing HANDSHAKE")
	// use our chanID to look up the channel, then get theirs from the channel
	p.sendHandshake(0, p.chans[ours].theirs, ws)
	p.closeChannel(ours)
}

func (p *Peer) sendHandshake(ours ChanID, theirs ChanID, ws *WrappedStream) {
	h := Handshake{C: ours}
	m := Msg{Op: handshake, Data: h}
	d := Datagram{ChanID: theirs, Msgs: []Msg{m}}
	sendDatagram(d, ws)
}

func chooseOurID() ChanID {
	// TODO
	return 1
}

// WrappedStream wraps a libp2p stream. We encode/decode whenever we
// write/read from a stream, so we can just carry the encoders
// and bufios with us
type WrappedStream struct {
	stream inet.Stream
	enc    multicodec.Encoder
	dec    multicodec.Decoder
	w      *bufio.Writer
	r      *bufio.Reader
}

// WrapStream takes a stream and complements it with r/w bufios and
// decoder/encoder. In order to write raw data to the stream we can use
// wrap.w.Write(). To encode something into it we can wrap.enc.Encode().
// Finally, we should wrap.w.Flush() to actually send the data. Handling
// incoming data works similarly with wrap.r.Read() for raw-reading and
// wrap.dec.Decode() to decode.
func WrapStream(s inet.Stream) *WrappedStream {
	reader := bufio.NewReader(s)
	writer := bufio.NewWriter(s)

	// Note that if these change, then the MarshalJSON/UnmarshalJSON functions for Msg
	// may no longer get called, which may mess up the codec for Msg.Data
	dec := json.Multicodec(false).Decoder(reader)
	enc := json.Multicodec(false).Encoder(writer)
	return &WrappedStream{
		stream: s,
		r:      reader,
		w:      writer,
		enc:    enc,
		dec:    dec,
	}
}

// NewBasicHost makes and initializes a basic host
func NewBasicHost(port int) host.Host {
	// Ignoring most errors for brevity
	// See echo example for more details and better implementation
	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	pid, _ := peer.IDFromPublicKey(pub)
	listen, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port))
	ps := ps.NewPeerstore()
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, pub)
	n, _ := swarm.NewNetwork(context.Background(),
		[]ma.Multiaddr{listen}, pid, ps, nil)
	return bhost.New(n)
}
