package wires

import (
	"context"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/huiscool/topic-wires/pb"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	msgio "github.com/libp2p/go-msgio"
	ma "github.com/multiformats/go-multiaddr"
)

const (
	TopicWiresProtocolID = protocol.ID("/topic-wires/1.0.0")
)

type TopicWires struct {
	rw      sync.RWMutex
	h       host.Host
	joined  map[string]struct{}
	topics  map[string]*PeerSet
	notif   Notifiee
	msghndl TopicMsgHandler
}

func NewTopicWires(h host.Host) (*TopicWires, error) {

	t := &TopicWires{
		rw:     sync.RWMutex{},
		h:      h,
		joined: make(map[string]struct{}),
		topics: make(map[string]*PeerSet),
		notif:  defaultNotif{},
	}

	t.h.SetStreamHandler(TopicWiresProtocolID, t.handleStream)
	h.Network().Notify((*topicWiresNotif)(t))
	return t, nil
}

func (t *TopicWires) Join(topic string) error {
	t.rw.Lock()
	defer t.rw.Unlock()
	// add topic
	t.joined[topic] = struct{}{}

	// send peer
	for _, peerid := range t.h.Network().Peers() {
		err := t.sendMsg(peerid, &pb.Message{
			MsgType: pb.Message_JOIN,
			Topics:  []string{topic},
			Data:    nil,
		})
		if err != nil {
			// TODO: log
		}
	}

	return nil
}

func (t *TopicWires) Leave(topic string) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	// notify our neighbors
	neighs, ok := t.topics[topic]
	if !ok {
		// have left
		return nil
	}
	for _, p := range neighs.Slice() {
		go func(p peer.ID) {
			err := t.sendMsg(p, &pb.Message{
				MsgType: pb.Message_LEAVE,
				Topics:  []string{topic},
				Data:    nil,
			})
			if err != nil {
				// TODO: log
			}
		}(p)
		go t.notif.HandleNeighDown(p, topic)
	}
	// delete topic
	delete(t.joined, topic)
	delete(t.topics, topic)

	return nil
}

func (t *TopicWires) Neighbors(topic string) []peer.ID {
	t.rw.RLock()
	defer t.rw.RUnlock()
	ps, ok := t.topics[topic]
	if !ok {
		return []peer.ID{}
	}
	return ps.Slice()
}

func (t *TopicWires) Connected() []peer.ID {
	t.rw.RLock()
	defer t.rw.RUnlock()
	return t.h.Network().Peers()
}

func (t *TopicWires) Topics() []string {
	t.rw.RLock()
	defer t.rw.RUnlock()
	return t.joinedTopics()
}

func (t *TopicWires) SetNotifiee(notifiee Notifiee) {
	t.rw.Lock()
	defer t.rw.Unlock()
	t.notif = notifiee
}

func (t *TopicWires) Close() error {
	t.rw.Lock()
	defer t.rw.Unlock()
	t.h.Network().StopNotify((*topicWiresNotif)(t))
	topics := make([]string, 0, len(t.joined))
	for topic := range t.joined {
		topics = append(topics, topic)
	}
	// send peer
	for _, peerid := range t.h.Network().Peers() {
		err := t.sendMsg(peerid, &pb.Message{
			MsgType: pb.Message_LEAVE,
			Topics:  topics,
			Data:    nil,
		})
		if err != nil {
			// TODO: log
		}
	}
	return nil
}

func (t *TopicWires) SendTopicMsg(topic string, p peer.ID, data []byte) error {
	return t.sendMsg(p, &pb.Message{
		MsgType: pb.Message_DATA,
		Topics:  []string{topic},
		Data:    data,
	})
}

type TopicMsgHandler func(topic string, from peer.ID, data []byte)

func (t *TopicWires) SetTopicMsgHandler(tmh TopicMsgHandler) {
	t.rw.Lock()
	defer t.rw.Unlock()
	t.msghndl = tmh
}

/*===========================================================================*/

func (t *TopicWires) joinedTopics() []string {
	out := make([]string, 0, len(t.joined))
	for topic := range t.joined {
		out = append(out, topic)
	}
	return out
}

func (t *TopicWires) sendMsg(to peer.ID, msg *pb.Message) error {
	msgBin, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	s, err := t.h.NewStream(context.Background(), to, TopicWiresProtocolID)
	if err != nil {
		return err
	}
	mrw := msgio.NewReadWriter(s)
	defer mrw.Close()
	err = mrw.WriteMsg(msgBin)
	if err != nil {
		return err
	}
	return nil
}

/*===========================================================================*/

func (t *TopicWires) handleStream(s network.Stream) {
	msg := &pb.Message{}
	mrw := msgio.NewReadWriter(s)
	defer s.Close()
	bin, err := mrw.ReadMsg()
	if err != nil {
		return
	}
	err = proto.Unmarshal(bin, msg)
	if err != nil {
		return
	}
	from := s.Conn().RemotePeer()
	switch msg.MsgType {
	case pb.Message_JOIN:
		t.handlePeerJoin(from, msg.Topics)
	case pb.Message_LEAVE:
		t.handlePeerLeave(from, msg.Topics)
	case pb.Message_JOIN_REPLY:
		t.handlePeerJoinReply(from, msg.Topics)
	case pb.Message_DATA:
		t.handlePeerData(from, msg.Topics, msg.Data)
	}
}

func (t *TopicWires) handlePeerDown(p peer.ID) {
	t.rw.Lock()
	defer t.rw.Unlock()
	for topic, ps := range t.topics {
		ps.Del(p)
		go t.notif.HandleNeighDown(p, topic)
	}
}

func (t *TopicWires) handlePeerJoin(p peer.ID, topics []string) {
	t.rw.Lock()
	var replyTopics = make([]string, 0, len(topics))
	for _, topic := range topics {
		ps, ok := t.topics[topic]
		if !ok {
			ps = NewPeerSet()
			t.topics[topic] = ps
		}
		ps.Add(p)
		if _, ok := t.joined[topic]; !ok {
			// received join request from p in a non-exist topic
			// add p in topics, but don't bother notifiee
			continue
		}
		replyTopics = append(replyTopics, topic)
		go t.notif.HandleNeighUp(p, topic)
	}
	t.rw.Unlock()
	t.sendMsg(p, &pb.Message{
		MsgType: pb.Message_JOIN_REPLY,
		Topics:  replyTopics,
		Data:    nil,
	})
}

func (t *TopicWires) handlePeerLeave(p peer.ID, topics []string) {
	t.rw.Lock()
	defer t.rw.Unlock()
	for _, topic := range topics {
		ps, ok := t.topics[topic]
		if !ok {
			// p has left
			continue
		}
		ps.Del(p)
		go t.notif.HandleNeighDown(p, topic)
	}
}

func (t *TopicWires) handlePeerJoinReply(p peer.ID, topics []string) {
	t.rw.Lock()
	defer t.rw.Unlock()
	for _, topic := range topics {
		ps, ok := t.topics[topic]
		if !ok {
			ps = NewPeerSet()
			t.topics[topic] = ps
		}
		ps.Add(p)
		go t.notif.HandleNeighUp(p, topic)
	}
}

func (t *TopicWires) handlePeerData(p peer.ID, topics []string, data []byte) {
	t.rw.RLock()
	hndl := t.msghndl
	t.rw.RUnlock()
	for _, topic := range topics {
		go hndl(topic, p, data)
	}
}

/*===========================================================================*/

type Notifiee interface {
	HandleNeighUp(neigh peer.ID, topic string)
	HandleNeighDown(neigh peer.ID, topic string)
}

type defaultNotif struct{}

func (d defaultNotif) HandleNeighUp(neigh peer.ID, topic string)   {}
func (d defaultNotif) HandleNeighDown(neigh peer.ID, topic string) {}

type NotifAdapter struct {
	HandleUp   func(neigh peer.ID, topic string)
	HandleDown func(neigh peer.ID, topic string)
}

func (na *NotifAdapter) HandleNeighUp(neigh peer.ID, topic string) {
	na.HandleUp(neigh, topic)
}
func (na *NotifAdapter) HandleNeighDown(neigh peer.ID, topic string) {
	na.HandleDown(neigh, topic)
}

/*===========================================================================*/
// TopicBasedOverlay as network notifiee

type topicWiresNotif TopicWires

func (tn *topicWiresNotif) OpenedStream(n network.Network, s network.Stream) {}
func (tn *topicWiresNotif) ClosedStream(n network.Network, s network.Stream) {}
func (tn *topicWiresNotif) Listen(n network.Network, a ma.Multiaddr)         {}
func (tn *topicWiresNotif) ListenClose(n network.Network, a ma.Multiaddr)    {}
func (tn *topicWiresNotif) Connected(n network.Network, c network.Conn) {
	t := ((*TopicWires)(tn))
	t.rw.RLock()
	defer t.rw.RUnlock()
	if len(t.joinedTopics()) == 0 {
		return
	}
	go func() {
		// err := t.sendMsg(c.RemotePeer(), t.joinedTopics(), pb.Message_JOIN)
		err := t.sendMsg(c.RemotePeer(), &pb.Message{
			MsgType: pb.Message_JOIN,
			Topics:  t.joinedTopics(),
			Data:    nil,
		})
		if err != nil {
			// log
		}
	}()
}

func (tn *topicWiresNotif) Disconnected(n network.Network, c network.Conn) {
	((*TopicWires)(tn)).handlePeerDown(c.RemotePeer())
}
