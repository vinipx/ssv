package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/bloxapp/ssv/ibft/proto"
)

func TestP2PNetworker(t *testing.T) {
	logger := zaptest.NewLogger(t)

	topic1 := uuid.New()
	topic2 := uuid.New()

	peer1, err := New(context.Background(), logger, topic1)
	require.NoError(t, err)

	peer2, err := New(context.Background(), logger, topic1)
	require.NoError(t, err)

	peer3, err := New(context.Background(), logger, topic1)
	require.NoError(t, err)

	peer4, err := New(context.Background(), logger, topic2)
	require.NoError(t, err)

	lambda := []byte("test-lambda")
	messageToBroadcast := &proto.SignedMessage{
		Message: &proto.Message{
			Type:   proto.RoundState_PrePrepare,
			Round:  1,
			Lambda: lambda,
			Value:  []byte("test-value"),
		},
	}

	peer1Chan := peer1.ReceivedMsgChan(1, lambda)
	peer2Chan := peer2.ReceivedMsgChan(2, lambda)
	peer3Chan := peer3.ReceivedMsgChan(3, lambda)
	peer4Chan := peer4.ReceivedMsgChan(4, lambda)

	time.Sleep(time.Second)

	err = peer1.Broadcast(lambda, messageToBroadcast)
	require.NoError(t, err)
	t.Log("message broadcasted")

	time.Sleep(time.Second)

	t.Run("peer 1 receives message", func(t *testing.T) {
		msgFromPeer1 := <-peer1Chan
		require.Equal(t, messageToBroadcast, msgFromPeer1)
	})

	t.Run("peer 2 receives message", func(t *testing.T) {
		msgFromPeer2 := <-peer2Chan
		require.Equal(t, messageToBroadcast, msgFromPeer2)
	})

	t.Run("peer 3 receives message", func(t *testing.T) {
		msgFromPeer3 := <-peer3Chan
		require.Equal(t, messageToBroadcast, msgFromPeer3)
	})

	t.Run("peer 4 ignores message", func(t *testing.T) {
		select {
		case <-peer4Chan:
			t.Error("unexpected own message")
		default:
		}
	})
}
