package core

import (
	"fmt"
)

func newStubNetworkPeer(id string) *Peer {
	p := NewPpspp()

	n := NewStubNetwork(id)

	return NewPeer(n, p)
}

// checkState checks that the peer's ProtocolState is equal to state for swarm sid for the remote peer
func checkState(sid SwarmID, p *Peer, remote PeerID, state ProtocolState) (bool, error) {
	foundState, err := p.P.ProtocolState(sid, remote)
	if err != nil {
		return false, fmt.Errorf("could not get state for %v: %v", p.ID(), err)
	}
	return (foundState == state), nil
}
