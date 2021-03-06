package local

import (
	"sync"
	"testing"

	"github.com/bloxapp/ssv/ibft/proto"
	"github.com/bloxapp/ssv/storage"
	"github.com/bloxapp/ssv/storage/inmem"
)

// Network implements network.Network interface
type Network struct {
	t      *testing.T
	replay *IBFTReplay
	l      map[uint64]*sync.Mutex
	c      map[uint64]chan *proto.SignedMessage
}

// ReceivedMsgChan implements network.Network interface
func (n *Network) ReceivedMsgChan(id uint64, lambda []byte) <-chan *proto.SignedMessage {
	c := make(chan *proto.SignedMessage)
	n.c[id] = c
	n.l[id] = &sync.Mutex{}
	return c
}

// Broadcast implements network.Network interface
func (n *Network) Broadcast(lambda []byte, signed *proto.SignedMessage) error {
	go func() {

		// verify node is not prevented from sending msgs
		for _, id := range signed.SignerIds {
			if !n.replay.CanSend(signed.Message.Type, signed.Message.Round, id) {
				return
			}
		}

		for i, c := range n.c {
			if !n.replay.CanReceive(signed.Message.Type, signed.Message.Round, i) {
				continue
			}

			n.l[i].Lock()
			c <- signed
			n.l[i].Unlock()
		}
	}()

	return nil
}

// BroadcastSignature broadcasts the given signature for the given lambda
func (n *Network) BroadcastSignature(lambda []byte, signature map[uint64][]byte) error {
	// TODO: Implement
	return nil
}

// ReceivedSignatureChan returns the channel with signatures
func (n *Network) ReceivedSignatureChan(lambda []byte) <-chan map[uint64][]byte {
	// TODO: Implement
	return nil
}

// IBFTReplay allows to script a precise scenario for every ibft node's behaviour each round
type IBFTReplay struct {
	Network *Network
	Storage storage.Storage
	scripts map[uint64]*RoundScript
	nodes   []uint64
}

// NewIBFTReplay is the constructor of IBFTReplay
func NewIBFTReplay(nodes map[uint64]*proto.Node) *IBFTReplay {
	ret := &IBFTReplay{
		Network: &Network{
			c: make(map[uint64]chan *proto.SignedMessage),
			l: make(map[uint64]*sync.Mutex),
		},
		Storage: inmem.New(),
		scripts: make(map[uint64]*RoundScript),
		nodes:   make([]uint64, len(nodes)),
	}
	ret.Network.replay = ret

	// set ids
	for k, v := range nodes {
		ret.nodes[k] = v.IbftId
	}

	return ret
}

// StartRound starts the given round
func (r *IBFTReplay) StartRound(round uint64) *RoundScript {
	r.scripts[round] = NewRoundScript(r, r.nodes)
	return r.scripts[round]
}

// CanSend returns true if message can be sent
func (r *IBFTReplay) CanSend(state proto.RoundState, round uint64, node uint64) bool {
	if v, ok := r.scripts[round]; ok {
		return v.CanSend(state, node)
	}
	return true
}

// CanReceive returns true if the message can be received
func (r *IBFTReplay) CanReceive(state proto.RoundState, round uint64, node uint64) bool {
	if v, ok := r.scripts[round]; ok {
		return v.CanSend(state, node)
	}
	return true
}

// RoundScript ...
type RoundScript struct {
	replay *IBFTReplay
	rules  map[proto.RoundState]map[uint64]bool // if true the node receives (and sends) all messages. False it doesn't
}

// NewRoundScript is the constructor of RoundScript
func NewRoundScript(r *IBFTReplay, nodes []uint64) *RoundScript {
	rules := make(map[proto.RoundState]map[uint64]bool)
	for _, t := range []proto.RoundState{proto.RoundState_PrePrepare, proto.RoundState_Prepare, proto.RoundState_Commit, proto.RoundState_ChangeRound} {
		rules[t] = make(map[uint64]bool)
		for _, id := range nodes {
			rules[t][id] = true
		}
	}
	return &RoundScript{
		rules:  rules,
		replay: r,
	}
}

// CanSend ...
func (r *RoundScript) CanSend(state proto.RoundState, node uint64) bool {
	return r.rules[state][node]
}

// CanReceive ...
func (r *RoundScript) CanReceive(state proto.RoundState, node uint64) bool {
	return r.rules[state][node]
}

// PreventMessages ...
func (r *RoundScript) PreventMessages(state proto.RoundState, nodes []uint64) *RoundScript {
	for _, id := range nodes {
		r.rules[state][id] = false
	}
	return r
}

// EndRound ...
func (r *RoundScript) EndRound() *IBFTReplay {
	return r.replay
}
