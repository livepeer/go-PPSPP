package core

import (
	"container/list"
	"errors"
	"fmt"
	"math/rand"

	"github.com/golang/glog"
)

// Protocol is the interface to the PPSPP logic
type Protocol interface {
	HandleDatagram(d *Datagram, id PeerID) error
	SetDatagramSender(f func(Datagram, PeerID) error)
	StartHandshake(remote PeerID, sid SwarmID) error
	SendClosingHandshake(remote PeerID, sid SwarmID) error
	ProtocolState(sid SwarmID, pid PeerID) (ProtocolState, error)
	AddSwarm(metadata SwarmMetadata)
	Swarm(id SwarmID) (*Swarm, error)
	AddLocalChunk(sid SwarmID, cid ChunkID, b []byte) error
	SendHave(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error
	SendRequest(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error
	SendData(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error
}

// MsgError is an error that happens while handling an incoming message
type MsgError struct {
	c    ChanID
	m    Msg
	info string
}

func (e MsgError) Error() string {
	return fmt.Sprintf("message error on channel %v: %v", e.c, e.info)
}

// SwarmID identifies a swarm
type SwarmID uint32

func (id SwarmID) String() string {
	return fmt.Sprintf("<SwarmID %d>", id)
}

// Opcode identifies the type of message
type Opcode uint8

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
	sw     SwarmID       // the swarm that this channel is communicating for
	theirs ChanID        // remote id to attach to outgoing datagrams on this channel
	state  ProtocolState // current state of the protocol on this channel
	remote PeerID        // PeerID of the remote peer
}

// Ppspp is a PPSPP implementation
type Ppspp struct {
	// all of this peer's channels, indexed by a local ChanID
	chans map[ChanID]*Chan

	// all of this peer's swarms, indexed by a global? SwarmID
	swarms map[SwarmID]*Swarm

	datagramSender func(Datagram, PeerID) error
}

// NewPpspp creates a new PPSPP protocol object
func NewPpspp() *Ppspp {

	// initially, there are no locally known swarms
	swarms := make(map[SwarmID](*Swarm))

	chans := make(map[ChanID](*Chan))
	// Special channel 0 is the reserved channel for incoming starting handshakes
	chans[0] = &Chan{}
	chans[0].state = Begin

	p := Ppspp{chans: chans, swarms: swarms}

	return &p
}

// HandleDatagram handles an incoming datagram from a remote peer with the given id
func (p *Ppspp) HandleDatagram(d *Datagram, id PeerID) error {
	glog.Infof("handling datagram from %v: %v\n", id, d)
	if len(d.Msgs) == 0 {
		return errors.New("no messages in datagram")
	}
	for _, msg := range d.Msgs {
		cid := d.ChanID
		_, ok := p.chans[cid]
		if !ok {
			return fmt.Errorf("channel %v not found", cid)
		}
		err := p.handleMsg(cid, msg, id)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetDatagramSender sets the datagram sender function
func (p *Ppspp) SetDatagramSender(f func(Datagram, PeerID) error) {
	p.datagramSender = f
}

func (p *Ppspp) sendDatagram(d Datagram, c ChanID) error {
	_, ok := p.chans[c]
	if !ok {
		return fmt.Errorf("could not find channel %v", c)
	}
	remote := p.chans[c].remote
	return p.datagramSender(d, remote)
}

// AddSwarm adds a swarm with a given ID
func (p *Ppspp) AddSwarm(metadata SwarmMetadata) {
	p.swarms[metadata.ID] = NewSwarm(metadata)
}

// Swarm returns the swarm at the given id
func (p *Ppspp) Swarm(id SwarmID) (*Swarm, error) {
	s, ok := p.swarms[id]
	if ok {
		return s, nil
	}
	return nil, fmt.Errorf("could not find swarm at id=%v", id)
}

func (p *Ppspp) handleMsg(c ChanID, m Msg, remote PeerID) error {
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

func (p *Ppspp) closeChannel(c ChanID) error {
	glog.Info("closing channel")
	delete(p.chans, c)
	return nil
}

// ProtocolState returns the current ProtocolState in a swarm for a given remote peer
// if this returns unknown state, check error for reason
func (p *Ppspp) ProtocolState(sid SwarmID, pid PeerID) (ProtocolState, error) {
	s, ok1 := p.swarms[sid]
	if !ok1 {
		return Unknown, fmt.Errorf("ProtocolState could not find swarm at sid=%v", sid)
	}
	cid, ok2 := s.chans[pid]
	if !ok2 {
		return Unknown, fmt.Errorf("ProtocolState could not find cid for sid=%v, pid=%v", sid, pid)
	}
	c, ok3 := p.chans[cid]
	if !ok3 {
		return Unknown, fmt.Errorf("ProtocolState could not find chan for sid=%v, pid=%v, cid=%v", sid, pid, cid)
	}
	return c.state, nil
}

// AddLocalChunk stores a chunk locally for the given swarm
func (p *Ppspp) AddLocalChunk(sid SwarmID, cid ChunkID, b []byte) error {
	c := &Chunk{ID: cid, B: b}
	swarm := p.swarms[sid]
	ref := swarm.ChunkSize()
	if size := len(b); ref != size {
		return fmt.Errorf("AddLocalChunk got size=%d, should be %d", size, ref)
	}
	return swarm.AddLocalChunk(cid, c)
}

// addChan adds a channel at the key ours
func (p *Ppspp) addChan(ours ChanID, sid SwarmID, theirs ChanID, state ProtocolState, remote PeerID) error {
	glog.Infof("addChan ours=%v, sid=%v, theirs=%v, state=%v, remote=%v", ours, sid, theirs, state, remote)

	if ours < 1 {
		return errors.New("cannot setup channel with ours<1")
	}

	// add the channel to the peer-level map
	_, ok := p.chans[ours]
	if ok {
		return fmt.Errorf("addChan error: channel %d already exists", ours)
	}
	p.chans[ours] = &Chan{sw: sid, theirs: theirs, state: state, remote: remote}

	// add the channel to the swarm-level map
	glog.Infof("adding channel %v to %v for %v", ours, sid, remote)
	sw, ok := p.swarms[sid]
	if !ok {
		return fmt.Errorf("no swarm exists at sid=%d", sid)
	}
	cid, ok := sw.chans[remote]
	if ok {
		return fmt.Errorf("addChan error: channel %d already exists for %v", cid, remote)
	}
	sw.chans[remote] = ours

	return nil
}

// chanIDForSwarmAndPeer is a convenience function to return the channel in the swarm for the given peer
func (p *Ppspp) chanIDForSwarmAndPeer(sid SwarmID, pid PeerID) (ChanID, bool) {
	// Check if we already have a channel for this remote peer in the swarm
	sw, ok := p.swarms[sid]
	if ok {
		cid, ok := sw.chans[pid]
		if ok {
			return cid, true
		}
	}
	return ChanID(0), false
}

func (p *Ppspp) randomUnusedChanID() ChanID {
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

// StubProtocol handles incoming datagrams by storing them
type StubProtocol struct {
	handledDatagrams list.List
}

func newStubProtocol() *StubProtocol {
	return &StubProtocol{}
}

// incoming is for holding the (datagram, id) pair that is sent to HandleDatagram
// only meant to be used by StubProtocol
type incoming struct {
	d  *Datagram
	id PeerID
}

// HandleDatagram stores incoming datagram, peerid pairs
func (p *StubProtocol) HandleDatagram(d *Datagram, id PeerID) error {
	p.handledDatagrams.PushBack(incoming{d, id})
	return nil
}

// ReadHandledDatagram pops the oldest handled datagram and returns it, along with the peer it came from
// Intended to be used by tests that want to inspect what is being handled by this stub protocol
func (p *StubProtocol) ReadHandledDatagram() (*Datagram, PeerID, error) {
	i, ok := p.handledDatagrams.Front().Value.(incoming)
	if !ok {
		return nil, StringPeerID{"0"}, fmt.Errorf("type assertion failed")
	}
	p.handledDatagrams.Remove(p.handledDatagrams.Front())
	return i.d, i.id, nil
}

// NumHandledDatagrams returns the number of unread handled datagrams
func (p *StubProtocol) NumHandledDatagrams() int {
	return p.handledDatagrams.Len()
}

// SetDatagramSender is a noop for StubProtocol
func (p *StubProtocol) SetDatagramSender(f func(Datagram, PeerID) error) {

}

// StartHandshake is a noop for StubProtocol
func (p *StubProtocol) StartHandshake(remote PeerID, sid SwarmID) error {
	return nil
}

// SendClosingHandshake is a noop for StubProtocol
func (p *StubProtocol) SendClosingHandshake(remote PeerID, sid SwarmID) error {
	return nil
}

// ProtocolState is a noop for StubProtocol
func (p *StubProtocol) ProtocolState(sid SwarmID, pid PeerID) (ProtocolState, error) {
	return Begin, nil
}

// AddSwarm is a noop for StubProtocol
func (p *StubProtocol) AddSwarm(metadata SwarmMetadata) {

}

// Swarm is a noop for StubProtocol
func (p *StubProtocol) Swarm(id SwarmID) (*Swarm, error) {
	return nil, nil
}

// AddLocalChunk is a noop for StubProtocol
func (p *StubProtocol) AddLocalChunk(sid SwarmID, cid ChunkID, b []byte) error {
	return nil
}

// SendHave is a noop for StubProtocol
func (p *StubProtocol) SendHave(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error {
	return nil
}

// SendRequest is a noop for StubProtocol
func (p *StubProtocol) SendRequest(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error {
	return nil
}

// SendData is a noop for StubProtocol
func (p *StubProtocol) SendData(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error {
	return nil
}
