package wires

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/huiscool/topic-wires/mock"
	"github.com/stretchr/testify/assert"
)

func mustNewTopicWires(h host.Host) *TopicWires {
	out, err := NewTopicWires(h)
	if err != nil {
		panic(err)
	}
	return out
}

func mustJoin(t *TopicWires, topic string) {
	err := t.Join(topic)
	if err != nil {
		panic(err)
	}
}

func mustLeave(t *TopicWires, topic string) {
	err := t.Leave(topic)
	if err != nil {
		panic(err)
	}
}

type assertNotif struct {
	upAssert   func(neigh peer.ID, topic string)
	downAssert func(neigh peer.ID, topic string)
}

func (an *assertNotif) HandleNeighUp(neigh peer.ID, topic string) {
	an.upAssert(neigh, topic)
}
func (an *assertNotif) HandleNeighDown(neigh peer.ID, topic string) {
	an.downAssert(neigh, topic)
}

func TestBasicJoinAndLeave(t *testing.T) {
	net := mock.NewMockNet()
	h1 := net.MustNewConnectedPeer()
	h2 := net.MustNewConnectedPeer()
	tbo1 := mustNewTopicWires(h1)
	tbo2 := mustNewTopicWires(h2)
	topic := "test"

	t.Run("join", func(t *testing.T) {
		var err error
		err = tbo1.Join(topic)
		assert.NoError(t, err)
		err = tbo2.Join(topic)
		assert.NoError(t, err)
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, []peer.ID{h2.ID()}, tbo1.Neighbors(topic))
		assert.Equal(t, []peer.ID{h1.ID()}, tbo2.Neighbors(topic))
	})
	t.Run("leave", func(t *testing.T) {
		var err error
		err = tbo1.Leave(topic)
		assert.NoError(t, err)
		err = tbo2.Leave(topic)
		assert.NoError(t, err)
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, []peer.ID{}, tbo1.Neighbors(topic))
		assert.Equal(t, []peer.ID{}, tbo2.Neighbors(topic))
	})
}

func TestConnectAfterJoin(t *testing.T) {
	net := mock.NewMockNet()
	h1 := net.MustNewLinkedPeer()
	h2 := net.MustNewLinkedPeer()
	tbo1 := mustNewTopicWires(h1)
	tbo2 := mustNewTopicWires(h2)
	topic := "test"

	var err error
	err = tbo1.Join(topic)
	assert.NoError(t, err)
	err = tbo2.Join(topic)
	assert.NoError(t, err)
	assert.Equal(t, []peer.ID{}, tbo1.Neighbors(topic))
	assert.Equal(t, []peer.ID{}, tbo2.Neighbors(topic))
	_, _ = net.ConnectPeers(h1.ID(), h2.ID())
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, []peer.ID{h2.ID()}, tbo1.Neighbors(topic))
	assert.Equal(t, []peer.ID{h1.ID()}, tbo2.Neighbors(topic))
}

func TestBasicJoinAndLeaveNotif(t *testing.T) {
	net := mock.NewMockNet()
	h1 := net.MustNewConnectedPeer()
	h2 := net.MustNewConnectedPeer()
	tbo1 := mustNewTopicWires(h1)
	tbo2 := mustNewTopicWires(h2)
	topic := "test"
	var okCnt int32 = 0
	assert1 := func(neigh peer.ID, ntopic string) {
		assert.Equal(t, h2.ID(), neigh)
		assert.Equal(t, topic, ntopic)
		atomic.AddInt32(&okCnt, 1)
	}
	assert2 := func(neigh peer.ID, ntopic string) {
		assert.Equal(t, h1.ID(), neigh)
		assert.Equal(t, topic, ntopic)
		atomic.AddInt32(&okCnt, 1)
	}
	notif1 := &assertNotif{
		upAssert:   assert1,
		downAssert: assert1,
	}
	notif2 := &assertNotif{
		upAssert:   assert2,
		downAssert: assert2,
	}
	tbo1.SetNotifiee(notif1)
	tbo2.SetNotifiee(notif2)

	mustJoin(tbo1, topic)
	mustJoin(tbo2, topic)
	mustLeave(tbo1, topic)
	mustLeave(tbo2, topic)
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(4), atomic.LoadInt32(&okCnt))
}

func TestConnectAfterJoinNotif(t *testing.T) {
	net := mock.NewMockNet()
	h1 := net.MustNewLinkedPeer()
	h2 := net.MustNewLinkedPeer()
	tbo1 := mustNewTopicWires(h1)
	tbo2 := mustNewTopicWires(h2)
	topic := "test"
	var okCnt int32 = 0
	assert1 := func(neigh peer.ID, ntopic string) {
		assert.Equal(t, h2.ID(), neigh)
		assert.Equal(t, topic, ntopic)
		atomic.AddInt32(&okCnt, 1)
	}
	assert2 := func(neigh peer.ID, ntopic string) {
		assert.Equal(t, h1.ID(), neigh)
		assert.Equal(t, topic, ntopic)
		atomic.AddInt32(&okCnt, 1)
	}
	notif1 := &assertNotif{
		upAssert:   assert1,
		downAssert: assert1,
	}
	notif2 := &assertNotif{
		upAssert:   assert2,
		downAssert: assert2,
	}
	tbo1.SetNotifiee(notif1)
	tbo2.SetNotifiee(notif2)

	mustJoin(tbo1, topic)
	mustJoin(tbo2, topic)
	_, _ = net.ConnectPeers(h1.ID(), h2.ID())
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(2), atomic.LoadInt32(&okCnt))
}
