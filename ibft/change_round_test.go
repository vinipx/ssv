package ibft

import (
	"encoding/json"
	"testing"

	"github.com/herumi/bls-eth-go-binary/bls"

	"github.com/stretchr/testify/require"

	"github.com/bloxapp/ssv/ibft/types"
)

func changeRoundDataToBytes(input *types.ChangeRoundData) []byte {
	ret, _ := json.Marshal(input)
	return ret
}
func bytesToChangeRoundData(input []byte) *types.ChangeRoundData {
	ret := &types.ChangeRoundData{}
	json.Unmarshal(input, ret)
	return ret
}

func signMsg(id uint64, sk *bls.SecretKey, msg *types.Message) types.SignedMessage {
	sig, _ := msg.Sign(sk)
	return types.SignedMessage{
		Message:   msg,
		Signature: sig.Serialize(),
		IbftId:    id,
	}
}

func TestRoundChangeInputValue(t *testing.T) {
	sks, nodes := generateNodes(4)
	i := &iBFTInstance{
		prepareMessages: types.NewMessagesContainer(),
		params: &types.InstanceParams{
			ConsensusParams: types.DefaultConsensusParams(),
			IbftCommittee:   nodes,
		},
		state: &types.State{
			Round:         1,
			PreparedRound: 0,
			PreparedValue: nil,
		},
	}

	// no prepared round
	byts, err := i.roundChangeInputValue()
	require.NoError(t, err)
	require.Nil(t, byts)

	// add votes
	i.prepareMessages.AddMessage(signMsg(1, sks[1], &types.Message{
		Type:   types.RoundState_Prepare,
		Round:  1,
		Lambda: []byte("lambda"),
		Value:  []byte("value"),
	}))
	i.prepareMessages.AddMessage(signMsg(2, sks[2], &types.Message{
		Type:   types.RoundState_Prepare,
		Round:  1,
		Lambda: []byte("lambda"),
		Value:  []byte("value"),
	}))

	// with some prepare votes but not enough
	byts, err = i.roundChangeInputValue()
	require.NoError(t, err)
	require.Nil(t, byts)

	// add more votes
	i.prepareMessages.AddMessage(signMsg(3, sks[3], &types.Message{
		Type:   types.RoundState_Prepare,
		Round:  1,
		Lambda: []byte("lambda"),
		Value:  []byte("value"),
	}))
	i.state.PreparedRound = 1
	i.state.PreparedValue = []byte("value")

	// with a prepared round
	byts, err = i.roundChangeInputValue()
	require.NoError(t, err)
	require.NotNil(t, byts)
	data := bytesToChangeRoundData(byts)
	require.EqualValues(t, 1, data.PreparedRound)
	require.EqualValues(t, []byte("value"), data.PreparedValue)

	// with a different prepared value
	i.state.PreparedValue = []byte("value2")
	byts, err = i.roundChangeInputValue()
	require.EqualError(t, err, "prepared value/ round is set but no quorum of prepare messages found")

	// with different prepared round
	i.state.PreparedRound = 2
	i.state.PreparedValue = []byte("value")
	byts, err = i.roundChangeInputValue()
	require.EqualError(t, err, "prepared value/ round is set but no quorum of prepare messages found")
}

func TestValidateChangeRoundMessage(t *testing.T) {
	sks, nodes := generateNodes(4)
	i := &iBFTInstance{
		params: &types.InstanceParams{
			ConsensusParams: types.DefaultConsensusParams(),
			IbftCommittee:   nodes,
		},
		state: &types.State{
			Round:         1,
			PreparedRound: 0,
			PreparedValue: nil,
		},
	}

	tests := []struct {
		name                string
		msg                 *types.Message
		signerId            uint64
		justificationSigIds []uint64
		expectedError       string
	}{
		{
			name:     "valid",
			signerId: 1,
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  1,
				Lambda: []byte("lambda"),
				Value:  nil,
			},
			expectedError: "",
		},
		{
			name:     "valid",
			signerId: 1,
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  2,
				Lambda: []byte("lambda"),
				Value:  nil,
			},
			expectedError: "",
		},
		{
			name:     "valid",
			signerId: 1,
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  3,
				Lambda: []byte("lambda"),
				Value:  nil,
			},
			expectedError: "",
		},
		{
			name:     "valid",
			signerId: 1,
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  3,
				Lambda: []byte("lambdas"),
				Value:  nil,
			},
			expectedError: "",
		},
		{
			name:                "valid justification",
			signerId:            1,
			justificationSigIds: []uint64{0, 1, 2},
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  3,
				Lambda: []byte("lambdas"),
				Value: changeRoundDataToBytes(&types.ChangeRoundData{
					PreparedRound: 2,
					PreparedValue: []byte("value"),
					JustificationMsg: &types.Message{
						Type:   types.RoundState_Prepare,
						Round:  2,
						Lambda: []byte("lambdas"),
						Value:  []byte("value"),
					},
					JustificationSig: nil,
					SignedIds:        []uint64{0, 1, 2},
				}),
			},
			expectedError: "",
		},
		{
			name:                "invalid justification msg type",
			signerId:            1,
			justificationSigIds: []uint64{0, 1, 2},
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  3,
				Lambda: []byte("lambdas"),
				Value: changeRoundDataToBytes(&types.ChangeRoundData{
					PreparedRound: 2,
					PreparedValue: []byte("value"),
					JustificationMsg: &types.Message{
						Type:   types.RoundState_PrePrepare,
						Round:  2,
						Lambda: []byte("lambdas"),
						Value:  []byte("value"),
					},
					JustificationSig: nil,
					SignedIds:        []uint64{0, 1, 2},
				}),
			},
			expectedError: "change round justification msg type not Prepare",
		},
		{
			name:                "invalid justification round",
			signerId:            1,
			justificationSigIds: []uint64{0, 1, 2},
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  3,
				Lambda: []byte("lambdas"),
				Value: changeRoundDataToBytes(&types.ChangeRoundData{
					PreparedRound: 2,
					PreparedValue: []byte("value"),
					JustificationMsg: &types.Message{
						Type:   types.RoundState_Prepare,
						Round:  3,
						Lambda: []byte("lambdas"),
						Value:  []byte("value"),
					},
					JustificationSig: nil,
					SignedIds:        []uint64{0, 1, 2},
				}),
			},
			expectedError: "change round justification round lower or equal to message round",
		},
		{
			name:                "invalid prepared and  justification round",
			signerId:            1,
			justificationSigIds: []uint64{0, 1, 2},
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  3,
				Lambda: []byte("lambdas"),
				Value: changeRoundDataToBytes(&types.ChangeRoundData{
					PreparedRound: 2,
					PreparedValue: []byte("value"),
					JustificationMsg: &types.Message{
						Type:   types.RoundState_Prepare,
						Round:  1,
						Lambda: []byte("lambdas"),
						Value:  []byte("value"),
					},
					JustificationSig: nil,
					SignedIds:        []uint64{0, 1, 2},
				}),
			},
			expectedError: "change round prepared round not equal to justification msg round",
		},
		{
			name:                "invalid justification instance",
			signerId:            1,
			justificationSigIds: []uint64{0, 1, 2},
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  3,
				Lambda: []byte("lambdas"),
				Value: changeRoundDataToBytes(&types.ChangeRoundData{
					PreparedRound: 2,
					PreparedValue: []byte("value"),
					JustificationMsg: &types.Message{
						Type:   types.RoundState_Prepare,
						Round:  2,
						Lambda: []byte("lambda"),
						Value:  []byte("value"),
					},
					JustificationSig: nil,
					SignedIds:        []uint64{0, 1, 2},
				}),
			},
			expectedError: "change round justification msg lambda not equal to msg lambda",
		},
		{
			name:                "valid justification",
			signerId:            1,
			justificationSigIds: []uint64{0, 1, 2},
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  3,
				Lambda: []byte("lambdas"),
				Value: changeRoundDataToBytes(&types.ChangeRoundData{
					PreparedRound: 2,
					PreparedValue: []byte("value"),
					JustificationMsg: &types.Message{
						Type:   types.RoundState_Prepare,
						Round:  2,
						Lambda: []byte("lambdas"),
						Value:  []byte("values"),
					},
					JustificationSig: nil,
					SignedIds:        []uint64{0, 1, 2},
				}),
			},
			expectedError: "change round prepared value not equal to justification msg value",
		},
		{
			name:                "invalid justification sig",
			signerId:            1,
			justificationSigIds: []uint64{0, 1},
			msg: &types.Message{
				Type:   types.RoundState_ChangeRound,
				Round:  3,
				Lambda: []byte("lambdas"),
				Value: changeRoundDataToBytes(&types.ChangeRoundData{
					PreparedRound: 2,
					PreparedValue: []byte("value"),
					JustificationMsg: &types.Message{
						Type:   types.RoundState_Prepare,
						Round:  2,
						Lambda: []byte("lambdas"),
						Value:  []byte("value"),
					},
					JustificationSig: nil,
					SignedIds:        []uint64{0, 1, 2},
				}),
			},
			expectedError: "change round justification signature doesn't verify",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// sign if needed
			if test.msg.Value != nil {
				var sig *bls.Sign
				data := bytesToChangeRoundData(test.msg.Value)
				for _, id := range test.justificationSigIds {
					s, err := data.JustificationMsg.Sign(sks[id])
					require.NoError(t, err)
					if sig == nil {
						sig = s
					} else {
						sig.Add(s)
					}
				}
				data.JustificationSig = sig.Serialize()
				test.msg.Value = changeRoundDataToBytes(data)
			}

			sig, err := test.msg.Sign(sks[test.signerId])
			require.NoError(t, err)

			err = i.validateChangeRoundMsg()(&types.SignedMessage{
				Message:   test.msg,
				Signature: sig.Serialize(),
				IbftId:    test.signerId,
			})
			if len(test.expectedError) > 0 {
				require.EqualError(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRoundChangeJustification(t *testing.T) {
	inputValue := changeRoundDataToBytes(&types.ChangeRoundData{
		PreparedRound: 1,
		PreparedValue: []byte("hello"),
	})

	i := &iBFTInstance{
		changeRoundMessages: types.NewMessagesContainer(),
		params: &types.InstanceParams{
			ConsensusParams: types.DefaultConsensusParams(),
			IbftCommittee: map[uint64]*types.Node{
				0: {IbftId: 0},
				1: {IbftId: 1},
				2: {IbftId: 2},
				3: {IbftId: 3},
			},
		},
		state: &types.State{
			Round:         1,
			PreparedRound: 0,
			PreparedValue: nil,
		},
	}

	// test no previous prepared round and no round change quorum
	//res, err := i.justifyRoundChange(2)
	//require.EqualError(t, err, "could not justify round change, did not find highest prepared")
	//require.False(t, res)

	i.changeRoundMessages.AddMessage(types.SignedMessage{
		Message: &types.Message{
			Type:   types.RoundState_ChangeRound,
			Round:  2,
			Lambda: []byte("lambda"),
			Value:  nil,
		},
		IbftId: 1,
	})
	i.changeRoundMessages.AddMessage(types.SignedMessage{
		Message: &types.Message{
			Type:   types.RoundState_ChangeRound,
			Round:  2,
			Lambda: []byte("lambda"),
			Value:  nil,
		},
		IbftId: 2,
	})
	i.changeRoundMessages.AddMessage(types.SignedMessage{
		Message: &types.Message{
			Type:   types.RoundState_ChangeRound,
			Round:  2,
			Lambda: []byte("lambda"),
			Value:  nil,
		},
		IbftId: 3,
	})

	// test no previous prepared round with round change quorum (no justification)
	res, err := i.justifyRoundChange(2)
	require.NoError(t, err)
	require.True(t, res)

	i.changeRoundMessages = types.NewMessagesContainer()
	i.changeRoundMessages.AddMessage(types.SignedMessage{
		Message: &types.Message{
			Type:   types.RoundState_ChangeRound,
			Round:  1,
			Lambda: []byte("lambda"),
			Value:  inputValue,
		},
		IbftId: 1,
	})
	i.changeRoundMessages.AddMessage(types.SignedMessage{
		Message: &types.Message{
			Type:   types.RoundState_ChangeRound,
			Round:  2,
			Lambda: []byte("lambda"),
			Value:  inputValue,
		},
		IbftId: 2,
	})
	i.changeRoundMessages.AddMessage(types.SignedMessage{
		Message: &types.Message{
			Type:   types.RoundState_ChangeRound,
			Round:  2,
			Lambda: []byte("lambda"),
			Value:  inputValue,
		},
		IbftId: 3,
	})

	// test no previous prepared round with round change quorum (with justification)
	res, err = i.justifyRoundChange(2)
	require.Errorf(t, err, "could not justify round change, did not receive quorum of prepare messages previously")
	require.False(t, res)

	i.state.PreparedRound = 1
	i.state.PreparedValue = []byte("hello")

	// test previously prepared round with round change quorum (with justification)
	res, err = i.justifyRoundChange(2)
	require.NoError(t, err)
	require.True(t, res)
}

func TestHighestPrepared(t *testing.T) {
	inputValue := []byte("input value")

	i := &iBFTInstance{
		changeRoundMessages: types.NewMessagesContainer(),
		params: &types.InstanceParams{
			ConsensusParams: types.DefaultConsensusParams(),
			IbftCommittee: map[uint64]*types.Node{
				0: {IbftId: 0},
				1: {IbftId: 1},
				2: {IbftId: 2},
				3: {IbftId: 3},
			},
		},
	}
	i.changeRoundMessages.AddMessage(types.SignedMessage{
		Message: &types.Message{
			Type:   types.RoundState_ChangeRound,
			Round:  3,
			Lambda: []byte("lambda"),
			Value: changeRoundDataToBytes(&types.ChangeRoundData{
				PreparedRound: 1,
				PreparedValue: inputValue,
			}),
		},
		IbftId: 1,
	})
	i.changeRoundMessages.AddMessage(types.SignedMessage{
		Message: &types.Message{
			Type:   types.RoundState_ChangeRound,
			Round:  3,
			Lambda: []byte("lambda"),
			Value: changeRoundDataToBytes(&types.ChangeRoundData{
				PreparedRound: 2,
				PreparedValue: append(inputValue, []byte("highest")...),
			}),
		},
		IbftId: 2,
	})

	// test one higher than other
	res, err := i.highestPrepared(3)
	require.NoError(t, err)
	require.EqualValues(t, 2, res.PreparedRound)
	require.EqualValues(t, append(inputValue, []byte("highest")...), res.PreparedValue)

	// test 2 equals
	i.changeRoundMessages.AddMessage(types.SignedMessage{
		Message: &types.Message{
			Type:   types.RoundState_ChangeRound,
			Round:  3,
			Lambda: []byte("lambda"),
			Value: changeRoundDataToBytes(&types.ChangeRoundData{
				PreparedRound: 2,
				PreparedValue: append(inputValue, []byte("highest")...),
			}),
		},
		IbftId: 2,
	})
	res, err = i.highestPrepared(3)
	require.NoError(t, err)
	require.EqualValues(t, 2, res.PreparedRound)
	require.EqualValues(t, append(inputValue, []byte("highest")...), res.PreparedValue)
}