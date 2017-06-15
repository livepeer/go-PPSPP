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

type MsgError struct {
	c    ChanID
	m    Msg
	info string
}

func (e MsgError) Error() string {
	return fmt.Sprintf("message error on channel %v: %v", e.c, e.info)
}

type ChanID uint

type Opcode uint8

const (
	handshake Opcode = 13 // weird number so it's easier to notice in debug info
)

type MsgData interface{}

type Handshake struct {
	C ChanID
	// TODO: swarm SwarmMetadata
	// TODO: peer capabilities
}

type Msg struct {
	Op   Opcode
	Data MsgData
}

func (m *Msg) UnmarshalJSON(b []byte) error {
	aux := &struct {
		Op   Opcode
		Data []byte
	}{}

	json0.Unmarshal(b, &aux)

	m.Op = aux.Op

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

func (m Msg) MarshalJSON() ([]byte, error) {
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

	j := make(map[string]interface{})
	j["Op"] = m.Op
	j["Data"] = b.Bytes()
	return json0.Marshal(j)
}

type Datagram struct {
	ChanID ChanID
	Msgs   []Msg
}

type ProtocolState uint

const (
	begin         ProtocolState = 0
	waitHandshake ProtocolState = 1 // waiting for ack of first handshake
	ready         ProtocolState = 2
)

type Chan struct {
	ours   ChanID // receiving channel id
	theirs ChanID // sending channel id
	state  ProtocolState
}

type Peer struct {
	chans map[ChanID](*Chan)
	h     host.Host
}

func NewPeer(port int) *Peer {
	chans := make(map[ChanID](*Chan))
	chans[0] = &Chan{0, 0, begin}
	h := NewBasicHost(port)
	return &Peer{chans: chans, h: h}
}

func (p *Peer) HandleStream(ws *WrappedStream) error {
	fmt.Println("handling stream")
	d, err := receiveDatagram(ws)
	fmt.Println("recvd Datagram")
	if err != nil {
		return err
	} else {
		return p.handleDatagram(d, ws)
	}
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
	return nil
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
		c.theirs = h.C
		p.sendReplyHandshake(c.theirs, ws)
		c.state = ready
	case waitHandshake:
		fmt.Println("in waitHandshake state")
		if h.C < 1 {
			return MsgError{c: c.ours, m: m, info: "HANDSHAKE cannot request channel ID 0"}
		}
		c.theirs = h.C
		c.state = ready
	default:
		return MsgError{c: c.ours, m: m, info: "bad channel state"}
	}
	return nil
}

func (p *Peer) setupChan(ours ChanID) error {
	if ours < 1 {
		return errors.New("cannot setup channel with ours<1")
	}
	p.chans[ours] = &Chan{ours: ours, state: begin}
	return nil
}

func (p *Peer) startHandshake(ws *WrappedStream) ChanID {
	fmt.Println("starting handshake")
	ours := chooseOurID()
	p.setupChan(ours)
	p.chans[ours].state = waitHandshake
	p.sendReqHandshake(ours, ws)
	return ours
}

func (p *Peer) sendReqHandshake(ours ChanID, ws *WrappedStream) {
	fmt.Println("Peer sending request HANDSHAKE")
	glog.Info("Peer sending request HANDSHAKE")
	p.sendHandshake(ours, 0, ws)
}

func (p *Peer) sendReplyHandshake(theirs ChanID, ws *WrappedStream) {
	fmt.Println("Peer sending reply HANDSHAKE")
	glog.Info("Peer sending reply HANDSHAKE")
	ours := chooseOurID()
	p.sendHandshake(ours, theirs, ws)
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

// streamWrap wraps a libp2p stream. We encode/decode whenever we
// write/read from a stream, so we can just carry the encoders
// and bufios with us
type WrappedStream struct {
	stream inet.Stream
	enc    multicodec.Encoder
	dec    multicodec.Decoder
	w      *bufio.Writer
	r      *bufio.Reader
}

// wrapStream takes a stream and complements it with r/w bufios and
// decoder/encoder. In order to write raw data to the stream we can use
// wrap.w.Write(). To encode something into it we can wrap.enc.Encode().
// Finally, we should wrap.w.Flush() to actually send the data. Handling
// incoming data works similarly with wrap.r.Read() for raw-reading and
// wrap.dec.Decode() to decode.
func WrapStream(s inet.Stream) *WrappedStream {
	reader := bufio.NewReader(s)
	writer := bufio.NewWriter(s)
	// This is where we pick our specific multicodec. In order to change the
	// codec, we only need to change this place.
	// See https://godoc.org/github.com/multiformats/go-multicodec/json
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
