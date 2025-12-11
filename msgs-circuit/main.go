package main

//go:generate go run verify-txs-field.go

import (
	"fmt"
	"strings"

	txscircuit "github.com/DongLieu/msg-circuit/txscircuit"

	"cosmossdk.io/math"
	banktypes "cosmossdk.io/x/bank/types"
	stakingtypes "cosmossdk.io/x/staking/types"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/gogoproto/proto"
)

func main() {
	// case1()
	case2()
}

type txsFieldAssertion struct {
	TypeURL     string
	FieldKey    byte
	FieldValue  []byte
	FieldOffset int
	BodyOffset  int
}

func case1() {
	fmt.Println("========== ZK PROOF FOR MULTI-MSG FIELD VERIFICATION ==========")
	fmt.Println()

	protoCodec := newBenchmarkProtoCodec()

	privKey := secp256k1.GenPrivKey()
	fromAddr := sdk.AccAddress(privKey.PubKey().Address()).String()
	toAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

	msgSend := &banktypes.MsgSend{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount:      sdk.NewCoins(sdk.NewCoin("uatom", math.NewInt(4242))),
	}

	valKey := secp256k1.GenPrivKey()
	valAddr := sdk.ValAddress(valKey.PubKey().Address()).String()
	msgDelegate := &stakingtypes.MsgDelegate{
		DelegatorAddress: fromAddr,
		ValidatorAddress: valAddr,
		Amount:           sdk.NewCoin("stake", math.NewInt(777)),
	}

	sendAny, err := codectypes.NewAnyWithValue(msgSend)
	if err != nil {
		panic(fmt.Errorf("wrap send: %w", err))
	}

	delegateAny, err := codectypes.NewAnyWithValue(msgDelegate)
	if err != nil {
		panic(fmt.Errorf("wrap delegate: %w", err))
	}

	txBytes := buildTxWithMessagesDemo(
		protoCodec,
		privKey,
		[]*codectypes.Any{sendAny, delegateAny},
	)
	fmt.Printf("Transaction created with %d bytes and %d messages\n", len(txBytes), len([]*codectypes.Any{sendAny, delegateAny}))
	fmt.Println()

	offsets, err := locateMessageOffsetsDemo(txBytes, len([]*codectypes.Any{sendAny, delegateAny}))
	if err != nil {
		panic(fmt.Errorf("locate offsets: %w", err))
	}

	sendFieldKey := byte((3 << 3) | 2)
	sendFieldValue, err := extractFieldValueDemo(sendAny.Value, sendFieldKey)
	if err != nil {
		panic(err)
	}
	sendFieldOffset, err := findFieldOffset(sendAny.Value, sendFieldKey, sendFieldValue)
	if err != nil {
		panic(err)
	}

	delegateFieldKey := byte((1 << 3) | 2)
	delegateFieldValue, err := extractFieldValueDemo(delegateAny.Value, delegateFieldKey)
	if err != nil {
		panic(err)
	}
	delegateFieldOffset, err := findFieldOffset(delegateAny.Value, delegateFieldKey, delegateFieldValue)
	if err != nil {
		panic(err)
	}

	assertions := []txsFieldAssertion{
		{
			TypeURL:     sendAny.TypeUrl,
			FieldKey:    sendFieldKey,
			FieldValue:  sendFieldValue,
			FieldOffset: sendFieldOffset,
			BodyOffset:  offsets[0],
		},
		{
			TypeURL:     delegateAny.TypeUrl,
			FieldKey:    delegateFieldKey,
			FieldValue:  delegateFieldValue,
			FieldOffset: delegateFieldOffset,
			BodyOffset:  offsets[1],
		},
	}

	configs := []txscircuit.MsgConfig{
		{
			MsgTypeLen:    len(sendAny.TypeUrl),
			FieldValueLen: len(sendFieldValue),
			MsgValueLen:   len(sendAny.Value),
		},
		{
			MsgTypeLen:    len(delegateAny.TypeUrl),
			FieldValueLen: len(delegateFieldValue),
			MsgValueLen:   len(delegateAny.Value),
		},
	}

	circuit := txscircuit.NewTxsFieldCircuit(len(txBytes), configs)

	fmt.Println("Compiling TxsFieldCircuit...")
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		panic(fmt.Errorf("compile circuit: %w", err))
	}
	fmt.Printf("Circuit compiled, constraints: %d\n\n", ccs.GetNbConstraints())

	fmt.Println("Running Groth16 setup...")
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(fmt.Errorf("setup: %w", err))
	}
	fmt.Println("Setup done!")

	fmt.Println("Preparing witness...")
	witness := prepareTxsWitness(txBytes, assertions, configs)
	fmt.Println("Witness ready!")

	fmt.Println("Generating proof...")
	fullWitness, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		panic(fmt.Errorf("full witness: %w", err))
	}
	proof, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		panic(fmt.Errorf("prove: %w", err))
	}
	fmt.Println("Proof generated!")

	fmt.Println("Verifying proof...")
	publicWitness, err := fullWitness.Public()
	if err != nil {
		panic(fmt.Errorf("public witness: %w", err))
	}
	if err := groth16.Verify(proof, vk, publicWitness); err != nil {
		panic(fmt.Errorf("verify: %w", err))
	}
	fmt.Println("✅ Proof verification SUCCEEDED!")
}

func newBenchmarkProtoCodec() *codec.ProtoCodec {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	stakingtypes.RegisterInterfaces(interfaceRegistry)
	return codec.NewProtoCodec(interfaceRegistry)
}

func buildTxWithMessagesDemo(
	protoCodec *codec.ProtoCodec,
	privKey *secp256k1.PrivKey,
	anyMsgs []*codectypes.Any,
) []byte {
	txBody := &txtypes.TxBody{
		Messages: anyMsgs,
		Memo:     strings.Repeat("txs-field-demo-", 10),
	}

	txBodyBytes, err := protoCodec.Marshal(txBody)
	if err != nil {
		panic(fmt.Errorf("marshal body: %w", err))
	}

	pubKeyAny, err := codectypes.NewAnyWithValue(privKey.PubKey())
	if err != nil {
		panic(fmt.Errorf("wrap pubkey: %w", err))
	}

	authInfo := &txtypes.AuthInfo{
		SignerInfos: []*txtypes.SignerInfo{
			{
				PublicKey: pubKeyAny,
				ModeInfo: &txtypes.ModeInfo{
					Sum: &txtypes.ModeInfo_Single_{
						Single: &txtypes.ModeInfo_Single{Mode: signingtypes.SignMode_SIGN_MODE_DIRECT},
					},
				},
				Sequence: 0,
			},
		},
		Fee: &txtypes.Fee{
			Amount:   sdk.NewCoins(sdk.NewCoin("uatom", math.NewInt(5000))),
			GasLimit: 300000,
		},
	}

	authInfoBytes, err := protoCodec.Marshal(authInfo)
	if err != nil {
		panic(fmt.Errorf("marshal auth info: %w", err))
	}

	signDoc := &txtypes.SignDoc{
		BodyBytes:     txBodyBytes,
		AuthInfoBytes: authInfoBytes,
		ChainId:       "txs-field-demo",
		AccountNumber: 0,
	}

	signDocBytes, err := proto.Marshal(signDoc)
	if err != nil {
		panic(fmt.Errorf("marshal sign doc: %w", err))
	}

	signature, err := privKey.Sign(signDocBytes)
	if err != nil {
		panic(fmt.Errorf("sign: %w", err))
	}

	txRaw := &txtypes.TxRaw{
		BodyBytes:     txBodyBytes,
		AuthInfoBytes: authInfoBytes,
		Signatures:    [][]byte{signature},
	}

	txBytes, err := protoCodec.Marshal(txRaw)
	if err != nil {
		panic(fmt.Errorf("marshal tx: %w", err))
	}

	return txBytes
}

func locateMessageOffsetsDemo(txBytes []byte, expected int) ([]int, error) {
	if len(txBytes) < 3 {
		return nil, fmt.Errorf("tx too short")
	}
	if txBytes[0] != 0x0a {
		return nil, fmt.Errorf("invalid body tag")
	}

	bodyLen, consumed, err := decodeVarintDemo(txBytes[1:])
	if err != nil {
		return nil, err
	}

	bodyStart := 1 + consumed
	if bodyStart+bodyLen > len(txBytes) {
		return nil, fmt.Errorf("body overflow")
	}

	offsets := make([]int, 0, expected)
	cursor := bodyStart
	limit := bodyStart + bodyLen
	for cursor < limit && len(offsets) < expected {
		if txBytes[cursor] != 0x0a {
			return nil, fmt.Errorf("unexpected tag at %d", cursor)
		}
		offsets = append(offsets, cursor)
		cursor++

		msgLen, consumed, err := decodeVarintDemo(txBytes[cursor:])
		if err != nil {
			return nil, err
		}
		cursor += consumed + msgLen
	}

	if len(offsets) != expected {
		return nil, fmt.Errorf("expected %d msgs, found %d", expected, len(offsets))
	}

	return offsets, nil
}

func decodeVarintDemo(data []byte) (int, int, error) {
	var value int
	var shift uint
	var i int
	for i = 0; i < len(data); i++ {
		b := data[i]
		value |= int(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, i + 1, nil
		}
		shift += 7
		if shift > 28 {
			return 0, 0, fmt.Errorf("varint too long")
		}
	}
	return 0, 0, fmt.Errorf("incomplete varint")
}

func extractFieldValueDemo(msgValue []byte, fieldKey byte) ([]byte, error) {
	for idx := 0; idx < len(msgValue); {
		key := msgValue[idx]
		idx++

		length, consumed, err := decodeVarintDemo(msgValue[idx:])
		if err != nil {
			return nil, fmt.Errorf("decode length: %w", err)
		}
		idx += consumed

		if idx+length > len(msgValue) {
			return nil, fmt.Errorf("field overruns message")
		}

		value := msgValue[idx : idx+length]
		if key == fieldKey {
			buf := make([]byte, len(value))
			copy(buf, value)
			return buf, nil
		}

		idx += length
	}

	return nil, fmt.Errorf("field 0x%x not found", fieldKey)
}

func findFieldOffset(msgValue []byte, fieldKey byte, fieldValue []byte) (int, error) {
	offset := 0
	for offset < len(msgValue) {
		if msgValue[offset] == fieldKey {
			// Found the field key, now verify the value matches
			offset++ // Skip key byte

			// Decode length
			length, consumed, err := decodeVarintDemo(msgValue[offset:])
			if err != nil {
				return 0, err
			}
			offset += consumed

			// Check if value matches
			if offset+length <= len(msgValue) {
				actualValue := msgValue[offset : offset+length]
				if len(actualValue) == len(fieldValue) {
					match := true
					for i := range actualValue {
						if actualValue[i] != fieldValue[i] {
							match = false
							break
						}
					}
					if match {
						// Return offset relative to start of msgValue
						// Need to return offset of the KEY byte
						return offset - consumed - 1, nil
					}
				}
			}
			offset += length
		} else {
			// Skip this field
			offset++ // Skip key
			length, consumed, err := decodeVarintDemo(msgValue[offset:])
			if err != nil {
				return 0, err
			}
			offset += consumed + length
		}
	}
	return 0, fmt.Errorf("field with key 0x%x and matching value not found", fieldKey)
}

func prepareTxsWitness(
	txBytes []byte,
	assertions []txsFieldAssertion,
	configs []txscircuit.MsgConfig,
) *txscircuit.TxsFieldCircuit {
	witness := txscircuit.NewTxsFieldCircuit(len(txBytes), configs)

	for i := range witness.TxBytes {
		if i < len(txBytes) {
			witness.TxBytes[i] = int(txBytes[i])
			witness.PublicTxBytes[i] = int(txBytes[i])
		} else {
			witness.TxBytes[i] = 0
			witness.PublicTxBytes[i] = 0
		}
	}

	for i, assertion := range assertions {
		copyBytes := func(dst []frontend.Variable, data []byte) {
			for j := range dst {
				if j < len(data) {
					dst[j] = int(data[j])
				} else {
					dst[j] = 0
				}
			}
		}
		copyBytes(witness.Msgs[i].MsgType, []byte(assertion.TypeURL))
		copyBytes(witness.Msgs[i].Field.Value, assertion.FieldValue)

		witness.Msgs[i].Field.Key = int(assertion.FieldKey)
		witness.Msgs[i].FieldOffset = assertion.FieldOffset
		witness.Msgs[i].BodyOffset = assertion.BodyOffset
	}

	return witness
}

func case2() {
	fmt.Println("========== ATTACK: DUPLICATE FIELD TEST ==========")
	fmt.Println()

	protoCodec := newBenchmarkProtoCodec()

	privKey := secp256k1.GenPrivKey()
	fromAddr := sdk.AccAddress(privKey.PubKey().Address()).String()
	toAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

	// Create normal MsgSend first
	msgSend := &banktypes.MsgSend{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount:      sdk.NewCoins(sdk.NewCoin("uatom", math.NewInt(4242))), // Public amount
	}

	// Marshal normally to get the structure
	normalValue, err := proto.Marshal(msgSend)
	if err != nil {
		panic(fmt.Errorf("marshal send: %w", err))
	}

	// ATTACK: Manually inject duplicate Amount field (field 3) with 10000uatom BEFORE the real one
	// Field 3 tag = (3 << 3) | 2 = 0x1A

	// Create the first (hidden) amount: 10000uatom
	hiddenCoin := sdk.NewCoin("uatom", math.NewInt(10000))
	hiddenCoinBytes, err := proto.Marshal(&hiddenCoin)
	if err != nil {
		panic(fmt.Errorf("marshal hidden coin: %w", err))
	}

	// Build malicious message value:
	// We need to inject the hidden field BEFORE the existing amount field
	// Strategy: Parse normalValue, find amount field, insert hidden field before it

	maliciousValue := make([]byte, 0, len(normalValue)+len(hiddenCoinBytes)+2)

	// Copy fields up to amount field (field 1 and 2)
	cursor := 0
	for cursor < len(normalValue) {
		tag := normalValue[cursor]
		cursor++

		// Decode length
		length, consumed, err := decodeVarintDemo(normalValue[cursor:])
		if err != nil {
			panic(err)
		}
		cursor += consumed

		// If this is amount field (tag 0x1A), inject hidden field BEFORE it
		if tag == 0x1A {
			// First, append the hidden amount field
			maliciousValue = append(maliciousValue, 0x1A) // Field 3 tag
			maliciousValue = append(maliciousValue, byte(len(hiddenCoinBytes)))
			maliciousValue = append(maliciousValue, hiddenCoinBytes...)

			fmt.Printf("✓ Injected HIDDEN amount field: 10000uatom (at offset %d)\n", len(maliciousValue)-len(hiddenCoinBytes)-2)

			// Then append the original amount field
			maliciousValue = append(maliciousValue, tag)
			maliciousValue = append(maliciousValue, byte(length))
			maliciousValue = append(maliciousValue, normalValue[cursor:cursor+length]...)

			fmt.Printf("✓ Kept PUBLIC amount field: 4242uatom (at offset %d)\n", len(maliciousValue)-length-2)

			cursor += length
			break
		} else {
			// Copy field as-is
			maliciousValue = append(maliciousValue, tag)
			maliciousValue = append(maliciousValue, byte(length))
			maliciousValue = append(maliciousValue, normalValue[cursor:cursor+length]...)
			cursor += length
		}
	}

	// Copy any remaining fields after amount
	if cursor < len(normalValue) {
		maliciousValue = append(maliciousValue, normalValue[cursor:]...)
	}

	fmt.Printf("✓ Malicious message created: %d bytes (original: %d bytes)\n", len(maliciousValue), len(normalValue))
	fmt.Println()

	// Create Any with malicious value
	sendAny := &codectypes.Any{
		TypeUrl: "/cosmos.bank.v1beta1.MsgSend",
		Value:   maliciousValue,
	}

	// Create normal delegate message
	valKey := secp256k1.GenPrivKey()
	valAddr := sdk.ValAddress(valKey.PubKey().Address()).String()
	msgDelegate := &stakingtypes.MsgDelegate{
		DelegatorAddress: fromAddr,
		ValidatorAddress: valAddr,
		Amount:           sdk.NewCoin("stake", math.NewInt(777)),
	}

	delegateAny, err := codectypes.NewAnyWithValue(msgDelegate)
	if err != nil {
		panic(fmt.Errorf("wrap delegate: %w", err))
	}

	txBytes := buildTxWithMessagesDemo(
		protoCodec,
		privKey,
		[]*codectypes.Any{sendAny, delegateAny},
	)
	fmt.Printf("Transaction created with %d bytes and %d messages\n", len(txBytes), 2)
	fmt.Println()

	offsets, err := locateMessageOffsetsDemo(txBytes, 2)
	if err != nil {
		panic(fmt.Errorf("locate offsets: %w", err))
	}

	// Extract the PUBLIC amount field (second occurrence, 4242uatom)
	sendFieldKey := byte((3 << 3) | 2) // Field 3
	sendFieldValue, err := extractFieldValueDemo(sendAny.Value, sendFieldKey)
	if err != nil {
		panic(err)
	}
	fmt.Printf("✓ Extracted PUBLIC amount field: %d bytes\n", len(sendFieldValue))

	// Find offset of PUBLIC amount (will find first occurrence = hidden one!)
	sendFieldOffset, err := findFieldOffset(sendAny.Value, sendFieldKey, sendFieldValue)
	if err != nil {
		panic(err)
	}
	fmt.Printf("✓ PUBLIC amount offset: %d\n", sendFieldOffset)
	fmt.Println()

	delegateFieldKey := byte((1 << 3) | 2)
	delegateFieldValue, err := extractFieldValueDemo(delegateAny.Value, delegateFieldKey)
	if err != nil {
		panic(err)
	}
	delegateFieldOffset, err := findFieldOffset(delegateAny.Value, delegateFieldKey, delegateFieldValue)
	if err != nil {
		panic(err)
	}

	assertions := []txsFieldAssertion{
		{
			TypeURL:     sendAny.TypeUrl,
			FieldKey:    sendFieldKey,
			FieldValue:  sendFieldValue,
			FieldOffset: sendFieldOffset,
			BodyOffset:  offsets[0],
		},
		{
			TypeURL:     delegateAny.TypeUrl,
			FieldKey:    delegateFieldKey,
			FieldValue:  delegateFieldValue,
			FieldOffset: delegateFieldOffset,
			BodyOffset:  offsets[1],
		},
	}

	configs := []txscircuit.MsgConfig{
		{
			MsgTypeLen:    len(sendAny.TypeUrl),
			FieldValueLen: len(sendFieldValue),
			MsgValueLen:   len(sendAny.Value),
		},
		{
			MsgTypeLen:    len(delegateAny.TypeUrl),
			FieldValueLen: len(delegateFieldValue),
			MsgValueLen:   len(delegateAny.Value),
		},
	}

	circuit := txscircuit.NewTxsFieldCircuit(len(txBytes), configs)

	fmt.Println("Compiling TxsFieldCircuit...")
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		panic(fmt.Errorf("compile circuit: %w", err))
	}
	fmt.Printf("Circuit compiled, constraints: %d\n\n", ccs.GetNbConstraints())

	fmt.Println("Running Groth16 setup...")
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(fmt.Errorf("setup: %w", err))
	}
	fmt.Println("Setup done!")

	fmt.Println("Preparing witness with PUBLIC amount (4242uatom)...")
	witness := prepareTxsWitness(txBytes, assertions, configs)
	fmt.Println("Witness ready!")

	fmt.Println()
	fmt.Println("🔴 ATTACK SCENARIO:")
	fmt.Println("   - Message has DUPLICATE field 3 (amount)")
	fmt.Println("   - First occurrence: 10000uatom (HIDDEN)")
	fmt.Println("   - Second occurrence: 4242uatom (PUBLIC in proof)")
	fmt.Println("   - Protobuf decoder will use LAST value = 4242uatom")
	fmt.Println("   - But attacker proves the second occurrence")
	fmt.Println()

	fmt.Println("Attempting to generate proof...")
	fullWitness, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		panic(fmt.Errorf("full witness: %w", err))
	}
	proof, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		fmt.Printf("❌ PROOF GENERATION FAILED (as expected)!\n")
		fmt.Printf("   Error: %v\n\n", err)
		fmt.Println("✅ SUCCESS: Circuit detected duplicate field and rejected the proof!")
		return
	}

	fmt.Println("⚠️  Proof generated! Verifying...")
	publicWitness, err := fullWitness.Public()
	if err != nil {
		panic(fmt.Errorf("public witness: %w", err))
	}
	if err := groth16.Verify(proof, vk, publicWitness); err != nil {
		fmt.Printf("❌ Verification failed: %v\n", err)
		fmt.Println("✅ Attack blocked at verification stage")
	} else {
		fmt.Println("🔴 CRITICAL: Proof verification SUCCEEDED!")
		fmt.Println("   Circuit accepted message with duplicate fields!")
		fmt.Println("   This is a security vulnerability!")
	}
}
