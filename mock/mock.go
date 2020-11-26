package mock

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	mnet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
)

// Net creates a mock network for testing.
type Net struct {
	mnet.Mocknet
}

// NewMockNet return a mock.Net instance
func NewMockNet() *Net {
	return &Net{
		Mocknet: mnet.New(context.Background()),
	}
}

var blackholeIP6 = net.ParseIP("100::")

// GenPeerWithMarshalablePrivKey is the alternative GenPeer(),
// to avoid the unmarshal failure when checking the sign
func (mn *Net) GenPeerWithMarshalablePrivKey() (host.Host, error) {
	sk, _, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		return nil, err
	}
	id, err := peer.IDFromPrivateKey(sk)
	if err != nil {
		return nil, err
	}
	suffix := id
	if len(id) > 8 {
		suffix = id[len(id)-8:]
	}
	ip := append(net.IP{}, blackholeIP6...)
	copy(ip[net.IPv6len-len(suffix):], suffix)
	a, err := ma.NewMultiaddr(fmt.Sprintf("/ip6/%s/tcp/4242", ip))
	if err != nil {
		return nil, fmt.Errorf("failed to create test multiaddr: %s", err)
	}

	h, err := mn.AddPeer(sk, a)
	if err != nil {
		return nil, err
	}

	return h, nil
}

// MustNewLinkedPeer returns a full-mesh-linked host
func (mn *Net) MustNewLinkedPeer() host.Host {
	h, err := mn.NewLinkedPeer()
	if err != nil {
		panic(err)
	}
	return h
}

// MustNewConnectedPeer returns a full-mesh-connected host
func (mn *Net) MustNewConnectedPeer() host.Host {
	h, err := mn.NewConnectedPeer()
	if err != nil {
		panic(err)
	}
	return h
}

// NewLinkedPeer returns a full-mesh-linked host
func (mn *Net) NewLinkedPeer() (host.Host, error) {
	h, err := mn.GenPeerWithMarshalablePrivKey()
	if err == nil {
		err = mn.LinkAllButSelf(h)
	}
	return h, err
}

// NewConnectedPeer returns a full-mesh-connected host
func (mn *Net) NewConnectedPeer() (host.Host, error) {
	h, err := mn.NewLinkedPeer()
	if err == nil {
		err = mn.ConnectAllButSelf(h)
	}
	return h, err
}

// LinkAllButSelf makes h links to every node in net
func (mn *Net) LinkAllButSelf(h host.Host) error {
	peers := mn.Peers()
	for _, p := range peers {
		if p == h.ID() {
			continue
		}
		if _, err := mn.LinkPeers(h.ID(), p); err != nil {
			return err
		}
	}
	return nil
}

// ConnectAllButSelf makes h connects to every node in net
func (mn *Net) ConnectAllButSelf(h host.Host) error {
	peers := mn.Peers()
	for _, p := range peers {
		if p == h.ID() {
			continue
		}
		if _, err := mn.ConnectPeers(h.ID(), p); err != nil {
			return err
		}
	}
	return nil
}
