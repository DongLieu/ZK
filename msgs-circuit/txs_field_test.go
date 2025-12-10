package main

// import (
// 	"fmt"
// 	"strings"
// 	"testing"

// 	"cosmossdk.io/math"
// 	banktypes "cosmossdk.io/x/bank/types"
// 	stakingtypes "cosmossdk.io/x/staking/types"

// 	txscircuit "github.com/DongLieu/msg-circuit/txscircuit"

// 	"github.com/consensys/gnark/frontend"
// 	"github.com/consensys/gnark/test"
// 	"github.com/cosmos/cosmos-sdk/codec"
// 	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
// 	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
// 	sdk "github.com/cosmos/cosmos-sdk/types"
// 	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
// 	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
// 	"github.com/cosmos/gogoproto/proto"
// )

// type msgSpec struct {
// 	fieldKey byte // protobuf tag (field_number<<3 | wire_type) của field length-delimited cần chứng minh
// 	buildAny func(protoCodec *codec.ProtoCodec, fromAddr string) (*codectypes.Any, error)
// }

// // go test -run TestTxsFieldCircuit_MultiMessages -timeout 90s
// func TestTxsFieldCircuit_MultiMessages(t *testing.T) {
// 	sendSpec := msgSpec{
// 		fieldKey: byte((3 << 3) | 2), // field 3 (amount) với wire-type length-delimited
// 		buildAny: func(protoCodec *codec.ProtoCodec, fromAddr string) (*codectypes.Any, error) {
// 			toAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
// 			msg := &banktypes.MsgSend{
// 				FromAddress: fromAddr,
// 				ToAddress:   toAddr,
// 				Amount:      sdk.NewCoins(sdk.NewCoin("uatom", math.NewInt(4242))),
// 			}
// 			return codectypes.NewAnyWithValue(msg)
// 		},
// 	}
// 	delegateSpec := msgSpec{
// 		fieldKey: byte((1 << 3) | 2), // field 1 (delegator_address) wire-type 2
// 		buildAny: func(protoCodec *codec.ProtoCodec, fromAddr string) (*codectypes.Any, error) {
// 			valKey := secp256k1.GenPrivKey()
// 			valAddr := sdk.ValAddress(valKey.PubKey().Address()).String()
// 			msg := &stakingtypes.MsgDelegate{
// 				DelegatorAddress: fromAddr,
// 				ValidatorAddress: valAddr,
// 				Amount:           sdk.NewCoin("stake", math.NewInt(777)),
// 			}
// 			return codectypes.NewAnyWithValue(msg)
// 		},
// 	}
// 	fundSpec := msgSpec{
// 		fieldKey: byte((1 << 3) | 2), // field 1 (depositor) wire-type 2
// 		buildAny: func(_ *codec.ProtoCodec, fromAddr string) (*codectypes.Any, error) {
// 			value, err := marshalFundCommunityPool(fromAddr, paddedCommunityPoolCoins())
// 			if err != nil {
// 				return nil, err
// 			}
// 			return &codectypes.Any{
// 				TypeUrl: "/cosmos.distribution.v1beta1.MsgFundCommunityPool",
// 				Value:   value,
// 			}, nil
// 		},
// 	}

// 	cases := []struct {
// 		name     string
// 		msgSpecs []msgSpec // thứ tự trong slice chính là thứ tự messages sẽ xuất hiện trong TxBody
// 	}{
// 		{
// 			name:     "MsgSend+MsgDelegate",
// 			msgSpecs: []msgSpec{sendSpec, delegateSpec},
// 		},
// 		{
// 			name:     "MsgSend+MsgFundCommunityPool",
// 			msgSpecs: []msgSpec{sendSpec, fundSpec},
// 		},
// 		{
// 			name:     "MsgDelegate+MsgFund+MsgSend",
// 			msgSpecs: []msgSpec{delegateSpec, fundSpec, sendSpec},
// 		},
// 	}

// 	for _, tc := range cases {
// 		tc := tc
// 		t.Run(tc.name, func(t *testing.T) {
// 			protoCodec := newTestProtoCodec()
// 			privKey := secp256k1.GenPrivKey()
// 			fromAddr := sdk.AccAddress(privKey.PubKey().Address()).String()

// 			anyMsgs := make([]*codectypes.Any, len(tc.msgSpecs))
// 			msgAssignments := make([]msgWitnessInput, len(tc.msgSpecs))
// 			configs := make([]txscircuit.MsgConfig, len(tc.msgSpecs))

// 			for i, spec := range tc.msgSpecs {
// 				anyMsg, err := spec.buildAny(protoCodec, fromAddr)
// 				if err != nil {
// 					t.Fatalf("build msg %d: %v", i, err)
// 				}
// 				fieldValue, err := extractFieldValue(anyMsg.Value, spec.fieldKey)
// 				if err != nil {
// 					t.Fatalf("extract field: %v", err)
// 				}
// 				fieldOffset, err := findFieldOffset(anyMsg.Value, spec.fieldKey, fieldValue)
// 				if err != nil {
// 					t.Fatalf("field offset: %v", err)
// 				}

// 				anyMsgs[i] = anyMsg
// 				configs[i] = txscircuit.MsgConfig{
// 					MsgTypeLen:    len(anyMsg.TypeUrl),
// 					FieldValueLen: len(fieldValue),
// 					MsgValueLen:   len(anyMsg.Value),
// 				}
// 				msgAssignments[i] = msgWitnessInput{
// 					TypeURL:     anyMsg.TypeUrl,
// 					FieldKey:    spec.fieldKey,
// 					FieldValue:  fieldValue,
// 					FieldOffset: fieldOffset,
// 				}
// 			}

// 			txBytes := buildTxWithMessages(t, protoCodec, privKey, anyMsgs)

// 			offsets, err := locateMessageOffsets(txBytes, len(anyMsgs))
// 			if err != nil {
// 				t.Fatalf("locate offsets: %v", err)
// 			}
// 			for i := range msgAssignments {
// 				msgAssignments[i].BodyOffset = offsets[i]
// 			}

// 			circuit := txscircuit.NewTxsFieldCircuit(len(txBytes), configs)
// 			assignment := txscircuit.NewTxsFieldCircuit(len(txBytes), configs)

// 			fillTxsAssignment(assignment, txBytes, msgAssignments)

// 			assert := test.NewAssert(t)
// 			assert.ProverSucceeded(circuit, assignment)
// 		})
// 	}
// }

// type msgWitnessInput struct {
// 	TypeURL     string
// 	FieldKey    byte
// 	FieldValue  []byte
// 	FieldOffset int
// 	BodyOffset  int
// }

// func fillTxsAssignment(
// 	witness *txscircuit.TxsFieldCircuit,
// 	txBytes []byte,
// 	msgs []msgWitnessInput,
// ) {
// 	for i := range witness.TxBytes {
// 		if i < len(txBytes) {
// 			witness.TxBytes[i] = int(txBytes[i])
// 			witness.PublicTxBytes[i] = int(txBytes[i])
// 		} else {
// 			witness.TxBytes[i] = 0
// 			witness.PublicTxBytes[i] = 0
// 		}
// 	}

// 	for i, msg := range msgs {
// 		assignBytes := func(dst []frontend.Variable, data []byte) {
// 			for j := range dst {
// 				if j < len(data) {
// 					dst[j] = int(data[j])
// 				} else {
// 					dst[j] = 0
// 				}
// 			}
// 		}

// 		assignBytes(witness.Msgs[i].MsgType, []byte(msg.TypeURL))
// 		assignBytes(witness.Msgs[i].Field.Value, msg.FieldValue)

// 		witness.Msgs[i].Field.Key = int(msg.FieldKey)
// 		witness.Msgs[i].FieldOffset = msg.FieldOffset
// 		witness.Msgs[i].BodyOffset = msg.BodyOffset
// 	}
// }

// func buildTxWithMessages(
// 	t *testing.T,
// 	protoCodec *codec.ProtoCodec,
// 	privKey *secp256k1.PrivKey,
// 	anyMsgs []*codectypes.Any,
// ) []byte {
// 	t.Helper()

// 	txBody := &txtypes.TxBody{
// 		Messages: anyMsgs,
// 		Memo:     strings.Repeat("field-proof-", 20),
// 	}

// 	txBodyBytes, err := protoCodec.Marshal(txBody)
// 	if err != nil {
// 		t.Fatalf("marshal body: %v", err)
// 	}

// 	pubKeyAny, err := codectypes.NewAnyWithValue(privKey.PubKey())
// 	if err != nil {
// 		t.Fatalf("wrap pubkey: %v", err)
// 	}

// 	authInfo := &txtypes.AuthInfo{
// 		SignerInfos: []*txtypes.SignerInfo{
// 			{
// 				PublicKey: pubKeyAny,
// 				ModeInfo: &txtypes.ModeInfo{
// 					Sum: &txtypes.ModeInfo_Single_{
// 						Single: &txtypes.ModeInfo_Single{Mode: signingtypes.SignMode_SIGN_MODE_DIRECT},
// 					},
// 				},
// 				Sequence: 0,
// 			},
// 		},
// 		Fee: &txtypes.Fee{
// 			Amount:   sdk.NewCoins(sdk.NewCoin("uatom", math.NewInt(500))),
// 			GasLimit: 200000,
// 		},
// 	}

// 	authInfoBytes, err := protoCodec.Marshal(authInfo)
// 	if err != nil {
// 		t.Fatalf("marshal auth info: %v", err)
// 	}

// 	signDoc := &txtypes.SignDoc{
// 		BodyBytes:     txBodyBytes,
// 		AuthInfoBytes: authInfoBytes,
// 		ChainId:       "multi-msg-chain",
// 		AccountNumber: 0,
// 	}

// 	signDocBytes, err := proto.Marshal(signDoc)
// 	if err != nil {
// 		t.Fatalf("marshal sign doc: %v", err)
// 	}

// 	sig, err := privKey.Sign(signDocBytes)
// 	if err != nil {
// 		t.Fatalf("sign: %v", err)
// 	}

// 	txRaw := &txtypes.TxRaw{
// 		BodyBytes:     txBodyBytes,
// 		AuthInfoBytes: authInfoBytes,
// 		Signatures:    [][]byte{sig},
// 	}

// 	txBytes, err := protoCodec.Marshal(txRaw)
// 	if err != nil {
// 		t.Fatalf("marshal tx: %v", err)
// 	}

// 	return txBytes
// }

// func locateMessageOffsets(txBytes []byte, expected int) ([]int, error) {
// 	if len(txBytes) < 3 {
// 		return nil, fmt.Errorf("tx too short")
// 	}
// 	if txBytes[0] != 0x0a {
// 		return nil, fmt.Errorf("invalid body tag")
// 	}

// 	bodyLen, bodyLenBytes, err := decodeVarint(txBytes[1:])
// 	if err != nil {
// 		return nil, err
// 	}
// 	bodyStart := 1 + bodyLenBytes
// 	if bodyStart+bodyLen > len(txBytes) {
// 		return nil, fmt.Errorf("body overflow")
// 	}

// 	offsets := make([]int, 0, expected)
// 	cursor := bodyStart
// 	limit := bodyStart + bodyLen
// 	for cursor < limit && len(offsets) < expected {
// 		if txBytes[cursor] != 0x0a {
// 			return nil, fmt.Errorf("unexpected tag at %d", cursor)
// 		}
// 		offsets = append(offsets, cursor)
// 		cursor++
// 		msgLen, consumed, err := decodeVarint(txBytes[cursor:])
// 		if err != nil {
// 			return nil, err
// 		}
// 		cursor += consumed + msgLen
// 	}

// 	if len(offsets) != expected {
// 		return nil, fmt.Errorf("expected %d msgs, found %d", expected, len(offsets))
// 	}

// 	return offsets, nil
// }
