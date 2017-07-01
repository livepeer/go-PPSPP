package core

import (
	"github.com/golang/glog"
	libp2ppeer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
)

// FIXME: where should this go?
const proto = "/goppspp/0.0.1"

// PeerID identifies a peer
// FIXME: need to hide the libp2p details behind Network inferface
type PeerID libp2ppeer.ID

func (id PeerID) String() string {
	// FIXME: need to hide the libp2p details behind Network inferface
	return libp2ppeer.ID(id).String()
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
	P Protocol
	n Network
}

// NewPeer makes and initializes a new peer
func NewPeer(port int) *Peer {

	prot := newPpspp()

	// This determines the network implementation (libp2p)
	n := newLibp2pNetwork(port)

	p := Peer{n: n, P: prot}

	// set the network's datagram handler
	p.n.SetDatagramHandler(prot.HandleDatagram)

	p.P.SetDatagramSender(n.SendDatagram)

	return &p
}

// ID returns the peer ID
func (p *Peer) ID() PeerID {
	return p.n.ID()
}

// FIXME: this is a hack for now... we don't want libp2p details here (e.g. ma.Multiaddr)
func (p *Peer) AddAddrs(remote PeerID, addrs []ma.Multiaddr) {
	p.n.AddAddrs(remote, addrs)
}
func (p *Peer) Addrs() []ma.Multiaddr {
	return p.n.Addrs()
}

// Connect creates a stream from p to the peer at id and sets a stream handler
func (p *Peer) Connect(id PeerID) error {
	glog.Infof("%s: Connecting to %s", p.ID(), id)
	return p.n.Connect(id)
}

// Disconnect closes the stream that p is using to connect to the peer at id
func (p *Peer) Disconnect(id PeerID) error {
	glog.Infof("%s: Disconnecting from %s", p.ID(), id)
	return p.n.Connect(id)
}
