package core

import (
	"fmt"

	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"

	"github.com/golang/glog"
)

// FIXME: where should this go?
const proto = "/goppspp/0.0.1"

// PeerID identifies a peer
type PeerID interface {
	String() string
}

// ChanID identifies a channel
type ChanID uint32

// Datagram holds a protocol datagram
type Datagram struct {
	ChanID ChanID
	Msgs   []Msg
}

// Peer implements protocol logic and underlying network
type Peer struct {

	// P handles protocol logic.
	P Protocol

	// n handles the underlying network.
	// private because no one should touch this except P
	n Network
}

// NewPeer makes a new peer
func NewPeer(n Network, p Protocol) *Peer {
	peer := Peer{n: n, P: p}

	// set the network's datagram handler
	peer.n.SetDatagramHandler(p.HandleDatagram)

	peer.P.SetDatagramSender(n.SendDatagram)

	return &peer
}

// NewLibp2pPeer makes a new peer with a libp2p network
func NewLibp2pPeer(port int, p Protocol) (*Peer, error) {
	// This determines the network implementation (libp2p)
	n, err := newLibp2pNetwork(port)
	if err != nil {
		return nil, err
	}

	return NewPeer(n, p), nil
}

// NewPpsppPeer makes a new peer with a ppspp protocol
func NewPpsppPeer(n Network) (*Peer, error) {
	return NewPeer(n, NewPpspp(n.ID())), nil
}

// NewLibp2pPpsppPeer makes a new peer with a libp2p network and Ppspp protocol
func NewLibp2pPpsppPeer(port int) (*Peer, error) {
	// This determines the network implementation (libp2p)
	n, err := newLibp2pNetwork(port)
	if err != nil {
		return nil, err
	}

	p := NewPpspp(n.ID())

	return NewPeer(n, p), nil
}

// ID returns the peer ID
func (p *Peer) ID() PeerID {
	return p.n.ID()
}

// AddAddrs adds multiaddresses for the remote peer to this peer's store
func (p *Peer) AddAddrs(remote PeerID, addrs []ma.Multiaddr) {
	p.n.AddAddrs(remote, addrs)
}

// Addrs returns multiaddresses for this peer
func (p *Peer) Addrs() []ma.Multiaddr {
	return p.n.Addrs()
}

// Connect creates a stream from p to the peer at id
func (p *Peer) Connect(id PeerID) error {
	glog.V(1).Infof("%v Connect to %s", p.ID(), id)
	return p.n.Connect(id)
}

// Disconnect closes the stream that p is using to connect to the peer at id
func (p *Peer) Disconnect(id PeerID) error {
	glog.V(1).Infof("%v Disconnect from %s", p.ID(), id)
	return p.n.Disconnect(id)
}

// StringPeerID is a simple implementation of PeerID using an underlying string
// Used for stubs
type StringPeerID struct {
	s string
}

func (s StringPeerID) String() string {
	return s.s
}

// messagize creates a Msg from the given data, infering the opcode from the dynamic type of data
func messagize(data interface{}) (*Msg, error) {
	var op Opcode
	switch data.(type) {
	case HaveMsg:
		op = Have
	case HandshakeMsg:
		op = Handshake
	case RequestMsg:
		op = Request
	case DataMsg:
		op = Data
	default:
		return nil, fmt.Errorf("bad data type")
	}
	m := Msg{Op: op, Data: data}
	return &m, nil
}

// datagramize creates a datagram with a single message
func datagramize(c ChanID, m *Msg) *Datagram {
	msgs := []Msg{*m}
	return &Datagram{ChanID: c, Msgs: msgs}
}
