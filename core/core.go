package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"

	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	libp2ppeer "github.com/libp2p/go-libp2p-peer"
	ps "github.com/libp2p/go-libp2p-peerstore"
	libp2pswarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
)

const proto = "/goppspp/0.0.1"

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
type ChanID uint32

// SwarmID identifies a swarm
type SwarmID uint32

func (id SwarmID) String() string {
	return fmt.Sprintf("<SwarmID %d>", id)
}

// Opcode identifies the type of message
type Opcode uint8

// PeerID identifies a peer
type PeerID libp2ppeer.ID

func (id PeerID) String() string {
	return libp2ppeer.ID(id).String()
}

// From the RFC:
//   +----------+------------------+
//   | Msg Type | Description      |
//   +----------+------------------+
//   | 0        | HANDSHAKE        |
//   | 1        | DATA             |
//   | 2        | ACK              |
//   | 3        | HAVE             |
//   | 4        | INTEGRITY        |
//   | 5        | PEX_RESv4        |
//   | 6        | PEX_REQ          |
//   | 7        | SIGNED_INTEGRITY |
//   | 8        | REQUEST          |
//   | 9        | CANCEL           |
//   | 10       | CHOKE            |
//   | 11       | UNCHOKE          |
//   | 12       | PEX_RESv6        |
//   | 13       | PEX_REScert      |
//   | 14-254   | Unassigned       |
//   | 255      | Reserved         |
//   +----------+------------------+
const (
	Handshake Opcode = 0
	Data      Opcode = 1
	Have      Opcode = 3
	Request   Opcode = 8
)

// MsgData holds the data payload of a message
type MsgData interface{}

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

// Datagram holds a protocol datagram
type Datagram struct {
	ChanID ChanID
	Msgs   []Msg
}

// ProtocolState is a per-channel state local to a peer
type ProtocolState uint

const (
	// Unknown state is used for errors.
	Unknown ProtocolState = 0

	// Begin is the initial state before a handshake.
	Begin ProtocolState = 1

	// WaitHandshake means waiting for ack of the first handshake.
	WaitHandshake ProtocolState = 2

	// Ready means the handshake is complete and the peer is ready for other types of messages.
	Ready ProtocolState = 3
)

// Chan holds the current state of a channel
type Chan struct {
	sw     SwarmID        // the swarm that this channel is communicating for
	theirs ChanID         // remote id to attach to outgoing datagrams on this channel
	state  ProtocolState  // current state of the protocol on this channel
	stream *WrappedStream // stream to use for sending and receiving datagrams on this channel
	remote PeerID         // PeerID of the remote peer
}

// Peer is currently just a couple of things related to a peer (as defined in the RFC)
type Peer struct {
	// libp2p Host interface
	h host.Host

	// all of this peer's channels, indexed by a local ChanID
	chans map[ChanID]*Chan

	// all of this peer's swarms, indexed by a global? SwarmID
	swarms map[SwarmID]*Swarm

	// all of this peer's streams, indexed by a global? peer.ID
	streams map[PeerID]*WrappedStream
}

// Host returns the host interface in the peer
func (p *Peer) Host() host.Host {
	return p.h
}

// AddSwarm adds a swarm with a given ID
func (p *Peer) AddSwarm(metadata SwarmMetadata) {
	p.swarms[metadata.ID] = NewSwarm(metadata)
}

// Swarm returns the swarm at the given id
func (p *Peer) Swarm(id SwarmID) (*Swarm, error) {
	s, ok := p.swarms[id]
	if ok {
		return s, nil
	}
	return nil, fmt.Errorf("could not find swarm at id=%v", id)
}

// NewPeer makes and initializes a new peer
func NewPeer(port int) *Peer {

	// initially, there are no locally known swarms
	swarms := make(map[SwarmID](*Swarm))

	chans := make(map[ChanID](*Chan))
	// Special channel 0 is the reserved channel for incoming starting handshakes
	chans[0] = &Chan{}
	chans[0].state = Begin

	// initially, no streams
	streams := make(map[PeerID](*WrappedStream))

	// Create a basic host to implement the libp2p Host interface
	h := NewBasicHost(port)

	p := Peer{chans: chans, h: h, swarms: swarms, streams: streams}

	// setup stream handler so we can immediately start receiving
	p.setupStreamHandler()

	return &p
}

// ID returns the peer ID
func (p *Peer) ID() PeerID {
	return PeerID(p.h.ID())
}

func (p *Peer) setupStreamHandler() {
	glog.Info("setting stream handler")
	p.h.SetStreamHandler(proto, func(s inet.Stream) {

		remote := s.Conn().RemotePeer()
		glog.Infof("%s received a stream from %s", p.ID(), remote)
		defer s.Close()
		ws := WrapStream(s)
		err := p.HandleStream(ws)
		glog.Infof("%v handled stream", p.ID())
		if err == io.EOF {
			glog.Infof("%v received EOF", p.ID())
			return
		} else if err != nil {
			glog.Fatal(err)
		}
	})
}

// HandleStream handles an incoming stream
// TODO: not sure how this works wrt multiple incoming datagrams
func (p *Peer) HandleStream(ws *WrappedStream) error {
	glog.Infof("%v handling stream", p.ID())
	for {
		d, err := p.receiveDatagram(ws)
		glog.Infof("%v recvd Datagram", p.ID())
		if err != nil {
			return err
		}
		if err = p.handleDatagram(d, ws); err != nil {
			return err
		}
	}
}

// receiveDatagram reads and decodes a datagram from the stream
func (p *Peer) receiveDatagram(ws *WrappedStream) (*Datagram, error) {
	glog.Infof("%v receiveDatagram", p.ID())
	if ws == nil {
		return nil, fmt.Errorf("%v receiveDatagram on nil *WrappedStream", p.h.ID())
	}
	var d Datagram
	err := ws.dec.Decode(&d)
	glog.Infof("%v decoded datagram %v", p.ID(), d)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// sendDatagram encodes and writes a datagram to the channel
func (p *Peer) sendDatagram(d Datagram, c ChanID) error {
	_, ok := p.chans[c]
	if !ok {
		return errors.New("could not find channel")
	}
	remote := p.chans[c].remote
	// s, err1 := p.h.NewStream(context.Background(), libp2ppeer.ID(remote), proto)
	// if err1 != nil {
	// 	return fmt.Errorf("sendDatagram: (chan %v) NewStream to %v: %v", c, remote, err1)
	// }
	// ws := WrapStream(s)
	ws, ok2 := p.streams[remote]
	if !ok2 {
		return fmt.Errorf("%v sendDatagram could not find stream for %v", p.ID(), remote)
	}

	glog.Infof("%v sending datagram %v\n", p.ID(), d)
	err2 := ws.enc.Encode(d)
	if err2 != nil {
		return fmt.Errorf("send datagram encode error %v", err2)
	}
	// Because output is buffered with bufio, we need to flush!
	err3 := ws.w.Flush()
	glog.Infof("%v flushed datagram", p.ID())
	if err3 != nil {
		return fmt.Errorf("send datagram flush error: %v", err3)
	}
	return nil
}

func (p *Peer) handleDatagram(d *Datagram, ws *WrappedStream) error {
	glog.Infof("%v handling datagram %v\n", p.ID(), d)
	if len(d.Msgs) == 0 {
		return errors.New("no messages in datagram")
	}
	for _, msg := range d.Msgs {
		cid := d.ChanID
		_, ok := p.chans[cid]
		if !ok {
			return errors.New("channel not found")
		}
		err := p.handleMsg(cid, msg, PeerID(ws.stream.Conn().RemotePeer()))
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Peer) handleMsg(c ChanID, m Msg, remote PeerID) error {
	switch m.Op {
	case Handshake:
		return p.handleHandshake(c, m, remote)
	case Have:
		return p.handleHave(c, m, remote)
	case Request:
		return p.handleRequest(c, m, remote)
	case Data:
		return p.handleData(c, m, remote)
	default:
		return MsgError{m: m, info: "bad opcode"}
	}
}

func (p *Peer) closeChannel(c ChanID) error {
	glog.Info("closing channel")
	delete(p.chans, c)
	return nil
}

// ProtocolState returns the current ProtocolState in a swarm for a given remote peer
// if this returns unknown state, check error for reason
func (p *Peer) ProtocolState(sid SwarmID, pid PeerID) (ProtocolState, error) {
	s, ok1 := p.swarms[sid]
	if !ok1 {
		return Unknown, fmt.Errorf("%v: ProtocolState could not find swarm at sid=%v", p.ID(), sid)
	}
	cid, ok2 := s.chans[pid]
	if !ok2 {
		return Unknown, fmt.Errorf("%v: ProtocolState could not find cid for sid=%v, pid=%v", p.ID(), sid, pid)
	}
	c, ok3 := p.chans[cid]
	if !ok3 {
		return Unknown, fmt.Errorf("%v: ProtocolState could not find chan for sid=%v, pid=%v, cid=%v", p.ID(), sid, pid, cid)
	}
	return c.state, nil
}

// addChan adds a channel at the key ours
func (p *Peer) addChan(ours ChanID, sid SwarmID, theirs ChanID, state ProtocolState, remote PeerID) error {
	glog.Infof("addChan ours=%v, sid=%v, theirs=%v, state=%v, remote=%v", ours, sid, theirs, state, remote)

	if ours < 1 {
		return errors.New("cannot setup channel with ours<1")
	}

	// add the channel to the peer-level map
	p.chans[ours] = &Chan{sw: sid, theirs: theirs, state: state, remote: remote}

	// add the channel to the swarm-level map
	glog.Infof("%v adding channel %v to swarm %v for %v", p.ID(), ours, sid, remote)
	sw, ok := p.swarms[sid]
	if !ok {
		return fmt.Errorf("no swarm exists at sid=%v", sid)
	}
	sw.chans[remote] = ours

	return nil
}

// NewBasicHost makes and initializes a basic host
func NewBasicHost(port int) host.Host {
	// Ignoring most errors for brevity
	// See echo example for more details and better implementation
	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	pid, _ := libp2ppeer.IDFromPublicKey(pub)
	listen, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port))
	ps := ps.NewPeerstore()
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, pub)
	n, _ := libp2pswarm.NewNetwork(context.Background(),
		[]ma.Multiaddr{listen}, pid, ps, nil)
	return bhost.New(n)
}

func (p *Peer) randomUnusedChanID() ChanID {
	// FIXME: seed should be based on time.now or something, but maybe
	// deterministic in some test/debug mode
	rand.Seed(486)
	maxUint32 := int(^uint32(0))
	for {
		c := ChanID(rand.Intn(maxUint32))
		_, ok := p.chans[c]
		if !ok && c != 0 {
			return c
		}
	}
}

// Connect/Disconnect functions ... for the optimization where we keep the streams open between datagrams
// I tried this and ended up falling back to the simpler approach where the stream is opened on the send,
// closed on the receive. Not yet sure exactly what to do here, but I think we can put this off for now.

// Connect creates a stream from p to the peer at id and sets a stream handler
func (p *Peer) Connect(id PeerID) (*WrappedStream, error) {
	glog.Infof("%s: Connecting to %s", p.ID(), id)
	stream, err := p.h.NewStream(context.Background(), libp2ppeer.ID(id), proto)
	if err != nil {
		return nil, err
	}

	ws := WrapStream(stream)

	p.streams[id] = ws

	return ws, nil
}

// Disconnect closes the stream that p is using to connect to the peer at id
func (p *Peer) Disconnect(id PeerID) error {
	glog.Infof("%s: Disconnecting from %s", p.ID(), id)
	ws, ok := p.streams[id]
	if ok {
		ws.stream.Close()
		return nil
	}
	return errors.New("disconnect error, no stream to close")
}
