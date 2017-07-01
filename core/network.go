package core

import (
	"context"
	"errors"
	"fmt"
	"io"

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

// Network handles interactions with the underlying network
type Network interface {

	// SendDatagram sends a datagram to the remote peer
	SendDatagram(d Datagram, remote PeerID) error

	// Connect connects to the remote peer and creates any io resources necessary for the connection
	Connect(remote PeerID) error

	// Disconnect disconnects from the remote peer and destroys any io resources created for the connection
	Disconnect(remote PeerID) error

	// ID returns the ID of this peer
	ID() PeerID

	// SetDatagramHandler sets the function that will be called on receipt of a new datagram
	// f gets called every time a new Datagram is received.
	SetDatagramHandler(f func(*Datagram, PeerID) error)

	// AddAddrs adds multiaddresses for the remote peer to this peer's store
	AddAddrs(id PeerID, addrs []ma.Multiaddr)

	// Addrs returns multiaddresses for this peer
	Addrs() []ma.Multiaddr
}

type libp2pNetwork struct {
	// all of this peer's streams, indexed by a global? peer.ID
	streams map[PeerID]*WrappedStream

	h host.Host
}

func newLibp2pNetwork(port int) *libp2pNetwork {
	streams := make(map[PeerID](*WrappedStream))
	h := newBasicHost(port)
	return &libp2pNetwork{streams: streams, h: h}
}

func (n *libp2pNetwork) ID() PeerID {
	return n.h.ID()
}

func (n *libp2pNetwork) AddAddrs(remote PeerID, addrs []ma.Multiaddr) {
	n.h.Peerstore().AddAddrs(remote.(libp2ppeer.ID), addrs, ps.PermanentAddrTTL)
}

func (n *libp2pNetwork) Addrs() []ma.Multiaddr {
	return n.h.Addrs()
}

func (n *libp2pNetwork) SetDatagramHandler(f func(*Datagram, PeerID) error) {
	glog.Info("setting stream handler")
	n.h.SetStreamHandler(proto, func(s inet.Stream) {

		remote := PeerID(s.Conn().RemotePeer())
		glog.Infof("%s received a stream from %s", n.ID(), remote)
		defer s.Close()
		ws := WrapStream(s)
		for {
			d, err := n.receiveDatagram(ws)
			glog.Infof("%v recvd Datagram", n.ID())
			if err == io.EOF {
				glog.Infof("%v received EOF", n.ID())
				break
			}
			if err != nil {
				glog.Fatal(err)
			}
			if err = f(d, remote); err != nil {
				glog.Fatal(err)
			}
		}
		glog.Infof("%v handled stream", n.ID())
	})
}

// SendDatagram encodes and writes a datagram to the channel
func (n *libp2pNetwork) SendDatagram(d Datagram, id PeerID) error {
	ws, ok := n.streams[id]
	if !ok {
		return fmt.Errorf("sendDatagram could not find stream at %v", id)
	}
	glog.Infof("sendDatagram sending datagram %v\n", d)
	err2 := ws.enc.Encode(d)
	if err2 != nil {
		return fmt.Errorf("send datagram encode error %v", err2)
	}
	// Because output is buffered with bufio, we need to flush!
	err3 := ws.w.Flush()
	glog.Info("sendDatagram flushed datagram")
	if err3 != nil {
		return fmt.Errorf("send datagram flush error: %v", err3)
	}
	return nil
}

// Connect creates a stream from p to the peer at id and sets a stream handler
func (n *libp2pNetwork) Connect(id PeerID) error {
	stream, err := n.h.NewStream(context.Background(), id.(libp2ppeer.ID), proto)
	if err != nil {
		return err
	}

	ws := WrapStream(stream)

	n.streams[id] = ws

	return nil
}

// Disconnect closes the stream that p is using to connect to the peer at id
func (n *libp2pNetwork) Disconnect(id PeerID) error {
	ws, ok := n.streams[id]
	if ok {
		ws.stream.Close()
		return nil
	}
	return errors.New("disconnect error, no stream to close")
}

// newBasicHost makes and initializes a basic host
func newBasicHost(port int) host.Host {
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

// receiveDatagram reads and decodes a datagram from the stream
func (n *libp2pNetwork) receiveDatagram(ws *WrappedStream) (*Datagram, error) {
	glog.Infof("%v receiveDatagram", n.ID())
	if ws == nil {
		return nil, fmt.Errorf("%v receiveDatagram on nil *WrappedStream", n.ID())
	}
	var d Datagram
	err := ws.dec.Decode(&d)
	glog.Infof("%v decoded datagram %v", n.ID(), d)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// StubNetwork stores all sent datagrams without sending them anywhere
type StubNetwork struct {
	id            StringPeerID
	SentDatagrams []*Datagram
}

func NewStubNetwork(s string) *StubNetwork {
	return &StubNetwork{id: StringPeerID{s}}
}

func (n *StubNetwork) SendDatagram(d Datagram, remote PeerID) error {
	n.SentDatagrams = append(n.SentDatagrams, &d)
	return nil
}

func (n *StubNetwork) Connect(remote PeerID) error {
	return nil

}

func (n *StubNetwork) Disconnect(remote PeerID) error {
	return nil

}

func (n *StubNetwork) ID() PeerID {
	return n.id
}

func (n *StubNetwork) SetDatagramHandler(f func(*Datagram, PeerID) error) {

}

func (n *StubNetwork) AddAddrs(id PeerID, addrs []ma.Multiaddr) {

}

func (n *StubNetwork) Addrs() []ma.Multiaddr {
	return nil
}
