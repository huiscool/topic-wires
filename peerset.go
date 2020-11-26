package wires

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
)

type PeerSet struct {
	rw    sync.RWMutex
	peers map[peer.ID]struct{}
}

func NewPeerSet() *PeerSet {
	return &PeerSet{
		rw:    sync.RWMutex{},
		peers: make(map[peer.ID]struct{}),
	}
}

func (ps *PeerSet) Add(p peer.ID) {
	ps.rw.Lock()
	defer ps.rw.Unlock()
	ps.peers[p] = struct{}{}
}

func (ps *PeerSet) Del(p peer.ID) {
	ps.rw.Lock()
	defer ps.rw.Unlock()
	delete(ps.peers, p)
}

func (ps *PeerSet) Equal(another *PeerSet) bool {
	ps.rw.RLock()
	defer ps.rw.RUnlock()
	another.rw.RLock()
	defer another.rw.RUnlock()
	if len(ps.peers) != len(another.peers) {
		return false
	}
	for pid := range ps.peers {
		if _, ok := another.peers[pid]; !ok {
			return false
		}
	}
	for pid := range another.peers {
		if _, ok := ps.peers[pid]; !ok {
			return false
		}
	}
	return true
}

func (ps *PeerSet) String() string {
	ps.rw.RLock()
	defer ps.rw.RUnlock()
	out := "["
	for p := range ps.peers {
		out += p.ShortString() + ","
	}
	out += "]"
	return out
}

func (ps *PeerSet) Slice() []peer.ID {
	if ps == nil {
		return []peer.ID{}
	}
	ps.rw.RLock()
	defer ps.rw.RUnlock()
	out := make([]peer.ID, 0, len(ps.peers))
	for p := range ps.peers {
		out = append(out, p)
	}
	return out
}
