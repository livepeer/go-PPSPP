package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"container/list"

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

	// emutex sync.Mutex
	// dmutex sync.Mutex
	mutex sync.Mutex
}

func newLibp2pNetwork(port int) (*libp2pNetwork, error) {
	streams := make(map[PeerID](*WrappedStream))
	h, err := newBasicHost(port)
	if err != nil {
		return nil, err
	}
	return &libp2pNetwork{streams: streams, h: h}, nil
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
	glog.V(1).Infof("%v setting stream handler", n.ID())
	n.h.SetStreamHandler(proto, func(s inet.Stream) {

		remote := PeerID(s.Conn().RemotePeer())
		glog.V(1).Infof("%v received a stream %v from %s", n.ID(), s, remote)
		defer s.Close()
		ws := WrapStream(s)
		for {
			d, err := n.receiveDatagram(ws)
			//glog.Fatal("foo")
			glog.V(1).Infof("%v recvd Datagram %v from stream", n.ID(), d)
			// if s != n.streams[remote].stream {
			// 	glog.Fatal("FIXME")
			// }
			if err == io.EOF {
				glog.V(2).Infof("%v received EOF", n.ID())
				break
			}
			if err != nil {
				glog.Fatal(err)
			}
			if err = f(d, remote); err != nil {
				glog.Fatal(err)
			}
		}
		glog.V(3).Infof("%v handled stream", n.ID())
	})
}

// SendDatagram encodes and writes a datagram to the channel
func (n *libp2pNetwork) SendDatagram(d Datagram, id PeerID) error {
	ws, ok := n.streams[id]
	if !ok {
		return fmt.Errorf("SendDatagram could not find stream at %v", id)
	}
	glog.V(1).Infof("%v SendDatagram sending datagram %v\n", n.ID(), d)
	n.lockEnc()
	glog.V(2).Infof("%v SendDatagram encoding\n", n.ID())
	if err := ws.enc.Encode(d); err != nil {
		return fmt.Errorf("SendDatagram encode error %v", err)
	}
	glog.V(2).Infof("%v SendDatagram flushing\n", n.ID())
	// Because output is buffered with bufio, we need to flush!
	if err := ws.w.Flush(); err != nil {
		return fmt.Errorf("SendDatagram flush error: %v", err)
	}
	glog.V(2).Infof("%v SendDatagram unlocking\n", n.ID())
	n.unlockEnc()
	glog.V(3).Infof("%v SendDatagram flushed datagram %v on stream 0x%x", n.ID(), d, ws.stream)
	return nil
}

// Connect creates a stream from p to the peer at id
func (n *libp2pNetwork) Connect(id PeerID) error {
	// _, ok := n.streams[id]
	// if ok {
	// 	// Connection already exists
	// 	_, err := n.h.NewStream(context.Background(), id.(libp2ppeer.ID), proto)
	// 	return err
	// }

	n.lockEnc()
	defer n.unlockEnc()
	stream, err := n.h.NewStream(context.Background(), id.(libp2ppeer.ID), proto)
	//stream, err := n.h.NewStream(context.Background(), id.(libp2ppeer.ID), "")
	if err != nil {
		return err
	}

	ws := WrapStream(stream)

	n.streams[id] = ws

	return nil
}

// Disconnect closes the stream that p is using to connect to the peer at id
func (n *libp2pNetwork) Disconnect(id PeerID) error {
	n.lockEnc()
	defer n.unlockEnc()
	ws, ok := n.streams[id]
	if ok {
		ws.stream.Close()
		return nil
	}
	return errors.New("disconnect error, no stream to close")
}

// newBasicHost makes and initializes a basic host
func newBasicHost(port int) (host.Host, error) {
	priv, pub, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		return nil, err
	}
	pid, err := libp2ppeer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}
	listen, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port))
	if err != nil {
		return nil, err
	}
	ps := ps.NewPeerstore()
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, pub)
	n, err := libp2pswarm.NewNetwork(context.Background(),
		[]ma.Multiaddr{listen}, pid, ps, nil)
	if err != nil {
		return nil, err
	}
	return bhost.New(n), nil
}

// receiveDatagram reads and decodes a datagram from the stream
func (n *libp2pNetwork) receiveDatagram(ws *WrappedStream) (*Datagram, error) {
	glog.V(3).Infof("%v receiveDatagram", n.ID())
	if ws == nil {
		return nil, fmt.Errorf("%v receiveDatagram on nil *WrappedStream", n.ID())
	}
	var d Datagram
	glog.V(2).Infof("%v receiveDatagram locking\n", n.ID())
	n.lockDec()
	defer n.unlockDec()
	glog.V(2).Infof("%v receiveDatagram decoding\n", n.ID())
	if err := ws.dec.Decode(&d); err != nil {
		glog.V(2).Infof("%v receiveDatagram Decode error %v\n", n.ID(), err)
		return nil, err
	}
	glog.V(2).Infof("%v receiveDatagram decoded datagram %v", n.ID(), d)
	return &d, nil
}

func (n *libp2pNetwork) lockEnc() {
	n.mutex.Lock()
}

func (n *libp2pNetwork) unlockEnc() {
	n.mutex.Unlock()
}

func (n *libp2pNetwork) lockDec() {
	n.mutex.Lock()
}

func (n *libp2pNetwork) unlockDec() {
	n.mutex.Unlock()
}

// StubNetwork stores all sent datagrams without sending them anywhere
type StubNetwork struct {
	id              StringPeerID
	sentDatagrams   list.List
	datagramHandler func(*Datagram, PeerID) error
}

// NewStubNetwork creates a new StubNetwork
func NewStubNetwork(s string) *StubNetwork {
	return &StubNetwork{id: StringPeerID{s}}
}

// SendDatagram stores the datagram without sending it anywhere
func (n *StubNetwork) SendDatagram(d Datagram, remote PeerID) error {
	n.sentDatagrams.PushBack(&d)
	return nil
}

// ReadSentDatagram pops the oldest sent datagram and returns it
// Intended to be used by tests that want to inspect what is being sent out on this stub network
func (n *StubNetwork) ReadSentDatagram() *Datagram {
	d := n.sentDatagrams.Front().Value.(*Datagram)
	n.sentDatagrams.Remove(n.sentDatagrams.Front())
	return d
}

// NumSentDatagrams returns the number of unread sent datagrams
func (n *StubNetwork) NumSentDatagrams() int {
	return n.sentDatagrams.Len()
}

// Connect is a nop for StubNetwork
func (n *StubNetwork) Connect(remote PeerID) error {
	return nil

}

// Disconnect is a nop for StubNetwork
func (n *StubNetwork) Disconnect(remote PeerID) error {
	return nil

}

// ID returns the PeerID for this peer on the network
func (n *StubNetwork) ID() PeerID {
	return n.id
}

// SetDatagramHandler sets the datagram handler function for incoming datagrams
func (n *StubNetwork) SetDatagramHandler(f func(*Datagram, PeerID) error) {
	n.datagramHandler = f
}

// InjectIncomingDatagram injects a datagram as if it came from the remote peer id
func (n *StubNetwork) InjectIncomingDatagram(d *Datagram, id PeerID) error {
	return n.datagramHandler(d, id)
}

// AddAddrs is a nop for StubNetwork
func (n *StubNetwork) AddAddrs(id PeerID, addrs []ma.Multiaddr) {

}

// Addrs is a nop for StubNetwork
func (n *StubNetwork) Addrs() []ma.Multiaddr {
	return nil
}
