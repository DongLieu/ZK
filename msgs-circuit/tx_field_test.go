package main

// import (
// 	"fmt"
// 	"strings"
// 	"testing"

// 	"cosmossdk.io/math"
// 	banktypes "cosmossdk.io/x/bank/types"
// 	stakingtypes "cosmossdk.io/x/staking/types"

// 	txcircuit "github.com/DongLieu/msg-circuit/txcircuit"

// 	"github.com/consensys/gnark/frontend"
// 	"github.com/consensys/gnark/test"
// 	"github.com/cosmos/cosmos-sdk/codec"
// 	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
// 	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
// 	"github.com/cosmos/cosmos-sdk/std"
// 	sdk "github.com/cosmos/cosmos-sdk/types"
// 	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
// 	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
// 	"github.com/cosmos/gogoproto/proto"
// )

// func TestTxFieldCircuitVariousMessages(t *testing.T) {
// 	cases := []struct {
// 		name     string
// 		fieldKey byte
// 		buildAny func(protoCodec *codec.ProtoCodec, signerAddr string) (*codectypes.Any, error)
// 	}{
// 		{
// 			name:     "MsgSendAmountCoin",
// 			fieldKey: byte((3 << 3) | 2), // field 3 = amount
// 			buildAny: func(protoCodec *codec.ProtoCodec, fromAddr string) (*codectypes.Any, error) {
// 				toAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
// 				msg := &banktypes.MsgSend{
// 					FromAddress: fromAddr,
// 					ToAddress:   toAddr,
// 					Amount:      sdk.NewCoins(sdk.NewCoin("uatom", math.NewInt(424242))),
// 				}
// 				return codectypes.NewAnyWithValue(msg)
// 			},
// 		},
// 		{
// 			name:     "MsgFundCommunityPoolDepositor",
// 			fieldKey: byte((1 << 3) | 2), // field 1 = depositor
// 			buildAny: func(_ *codec.ProtoCodec, fromAddr string) (*codectypes.Any, error) {
// 				value, err := marshalFundCommunityPool(fromAddr, paddedCommunityPoolCoins())
// 				if err != nil {
// 					return nil, err
// 				}
// 				return &codectypes.Any{
// 					TypeUrl: "/cosmos.distribution.v1beta1.MsgFundCommunityPool",
// 					Value:   value,
// 				}, nil
// 			},
// 		},
// 		{
// 			name:     "MsgDelegateDelegator",
// 			fieldKey: byte((1 << 3) | 2), // field 1 = delegator_address
// 			buildAny: func(protoCodec *codec.ProtoCodec, fromAddr string) (*codectypes.Any, error) {
// 				valKey := secp256k1.GenPrivKey()
// 				valAddr := sdk.ValAddress(valKey.PubKey().Address()).String()
// 				msg := &stakingtypes.MsgDelegate{
// 					DelegatorAddress: fromAddr,
// 					ValidatorAddress: valAddr,
// 					Amount:           sdk.NewCoin("stake", math.NewInt(777)),
// 				}
// 				return codectypes.NewAnyWithValue(msg)
// 			},
// 		},
// 	}

// 	for _, tc := range cases {
// 		tc := tc
// 		t.Run(tc.name, func(t *testing.T) {
// 			protoCodec := newTestProtoCodec()

// 			privKey := secp256k1.GenPrivKey()
// 			fromAddr := sdk.AccAddress(privKey.PubKey().Address()).String()
// 			anyMsg, err := tc.buildAny(protoCodec, fromAddr)
// 			if err != nil {
// 				t.Fatalf("build any: %v", err)
// 			}

// 			txBytes, anyMsg := buildTxWithAny(t, protoCodec, privKey, anyMsg)

// 			fieldValue, err := extractFieldValue(anyMsg.Value, tc.fieldKey)
// 			if err != nil {
// 				t.Fatalf("failed to extract field value: %v", err)
// 			}

// 			fieldOffset, err := findFieldOffset(anyMsg.Value, tc.fieldKey, fieldValue)
// 			if err != nil {
// 				t.Fatalf("unable to locate field offset: %v", err)
// 			}

// 			circuit := txcircuit.NewTxFieldCircuit(
// 				len(txBytes),
// 				len(anyMsg.TypeUrl),
// 				len(fieldValue),
// 				len(anyMsg.Value),
// 			)

// 			assignment := txcircuit.NewTxFieldCircuit(
// 				len(txBytes),
// 				len(anyMsg.TypeUrl),
// 				len(fieldValue),
// 				len(anyMsg.Value),
// 			)

// 			fillAssignment(assignment, txBytes, anyMsg.TypeUrl, tc.fieldKey, fieldValue, fieldOffset)

// 			assert := test.NewAssert(t)
// 			assert.ProverSucceeded(circuit, assignment)
// 		})
// 	}
// }

// func newTestProtoCodec() *codec.ProtoCodec {
// 	interfaceRegistry := codectypes.NewInterfaceRegistry()
// 	std.RegisterInterfaces(interfaceRegistry)
// 	banktypes.RegisterInterfaces(interfaceRegistry)
// 	stakingtypes.RegisterInterfaces(interfaceRegistry)
// 	return codec.NewProtoCodec(interfaceRegistry)
// }

// func buildTxWithAny(
// 	t *testing.T,
// 	protoCodec *codec.ProtoCodec,
// 	privKey *secp256k1.PrivKey,
// 	anyMsg *codectypes.Any,
// ) ([]byte, *codectypes.Any) {
// 	t.Helper()

// 	txBody := &txtypes.TxBody{
// 		Messages: []*codectypes.Any{anyMsg},
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
// 						Single: &txtypes.ModeInfo_Single{
// 							Mode: signingtypes.SignMode_SIGN_MODE_DIRECT,
// 						},
// 					},
// 				},
// 				Sequence: 0,
// 			},
// 		},
// 		Fee: &txtypes.Fee{
// 			Amount:   sdk.NewCoins(sdk.NewCoin("uatom", math.NewInt(50))),
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
// 		ChainId:       "field-test-chain",
// 		AccountNumber: 0,
// 	}

// 	signDocBytes, err := proto.Marshal(signDoc)
// 	if err != nil {
// 		t.Fatalf("marshal signdoc: %v", err)
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
// 		t.Fatalf("marshal tx raw: %v", err)
// 	}

// 	return txBytes, anyMsg
// }

// func extractFieldValue(msgValue []byte, fieldKey byte) ([]byte, error) {
// 	for idx := 0; idx < len(msgValue); {
// 		key := msgValue[idx]
// 		idx++

// 		length, consumed, err := decodeVarint(msgValue[idx:])
// 		if err != nil {
// 			return nil, fmt.Errorf("decode length: %w", err)
// 		}
// 		idx += consumed

// 		if idx+length > len(msgValue) {
// 			return nil, fmt.Errorf("field overruns message")
// 		}

// 		value := msgValue[idx : idx+length]
// 		if key == fieldKey {
// 			buf := make([]byte, len(value))
// 			copy(buf, value)
// 			return buf, nil
// 		}

// 		idx += length
// 	}

// 	return nil, fmt.Errorf("field 0x%x not found", fieldKey)
// }

// func decodeVarint(data []byte) (int, int, error) {
// 	var (
// 		value int
// 		shift uint
// 		i     int
// 	)
// 	for i = 0; i < len(data); i++ {
// 		b := data[i]
// 		value |= int(b&0x7f) << shift
// 		if b&0x80 == 0 {
// 			return value, i + 1, nil
// 		}
// 		shift += 7
// 		if shift > 28 {
// 			return 0, 0, fmt.Errorf("varint too long")
// 		}
// 	}
// 	return 0, 0, fmt.Errorf("incomplete varint")
// }

// func fillAssignment(
// 	witness *txcircuit.TxFieldCircuit,
// 	txBytes []byte,
// 	msgType string,
// 	fieldKey byte,
// 	fieldValue []byte,
// 	fieldOffset int,
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

// 	assignBytes := func(dst []frontend.Variable, data []byte) {
// 		for i := range dst {
// 			if i < len(data) {
// 				dst[i] = int(data[i])
// 			} else {
// 				dst[i] = 0
// 			}
// 		}
// 	}

// 	assignBytes(witness.MsgType, []byte(msgType))
// 	assignBytes(witness.Field.Value, fieldValue)

// 	witness.Field.Key = int(fieldKey)
// 	witness.FieldOffset = fieldOffset
// }

// func marshalFundCommunityPool(depositor string, coins sdk.Coins) ([]byte, error) {
// 	value := make([]byte, 0, len(depositor)+32)
// 	value = append(value, byte((1<<3)|2)) // field 1 tag
// 	value = append(value, encodeVarint(len(depositor))...)
// 	value = append(value, []byte(depositor)...)

// 	for _, coin := range coins {
// 		coinBytes, err := proto.Marshal(&coin)
// 		if err != nil {
// 			return nil, fmt.Errorf("marshal coin: %w", err)
// 		}
// 		value = append(value, byte((2<<3)|2))
// 		value = append(value, encodeVarint(len(coinBytes))...)
// 		value = append(value, coinBytes...)
// 	}

// 	return value, nil
// }

// func paddedCommunityPoolCoins() sdk.Coins {
// 	return sdk.NewCoins(
// 		sdk.NewCoin("uosmo", math.NewInt(55)),
// 		sdk.NewCoin("ustake", math.NewInt(123456789)),
// 		sdk.NewCoin("padcoin0000000000001", math.NewInt(987654321)),
// 		sdk.NewCoin("padcoin0000000000002", math.NewInt(876543210)),
// 		sdk.NewCoin("padcoin0000000000003", math.NewInt(765432109)),
// 	)
// }
