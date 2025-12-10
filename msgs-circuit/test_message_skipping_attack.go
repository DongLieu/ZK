package main

import (
	"fmt"

	txscircuit "github.com/DongLieu/msg-circuit/txscircuit"

	"cosmossdk.io/math"
	banktypes "cosmossdk.io/x/bank/types"
	stakingtypes "cosmossdk.io/x/staking/types"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Test: Attack #4 - Message Skipping Attack
// Scenario: Tx has 3 messages, but attacker only proves 2 messages (skipping the first one)
func testMessageSkippingAttack() {
	fmt.Println("========== TEST: MESSAGE SKIPPING ATTACK ==========")
	fmt.Println()

	protoCodec := newBenchmarkProtoCodec()

	privKey := secp256k1.GenPrivKey()
	fromAddr := sdk.AccAddress(privKey.PubKey().Address()).String()
	toAddr := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

	// Create 3 messages
	msgSend1 := &banktypes.MsgSend{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount:      sdk.NewCoins(sdk.NewCoin("uatom", math.NewInt(9999))), // MALICIOUS - large amount
	}

	valKey := secp256k1.GenPrivKey()
	valAddr := sdk.ValAddress(valKey.PubKey().Address()).String()
	msgDelegate := &stakingtypes.MsgDelegate{
		DelegatorAddress: fromAddr,
		ValidatorAddress: valAddr,
		Amount:           sdk.NewCoin("stake", math.NewInt(777)),
	}

	msgSend2 := &banktypes.MsgSend{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount:      sdk.NewCoins(sdk.NewCoin("uatom", math.NewInt(100))), // LEGITIMATE - small amount
	}

	sendAny1, _ := codectypes.NewAnyWithValue(msgSend1)
	delegateAny, _ := codectypes.NewAnyWithValue(msgDelegate)
	sendAny2, _ := codectypes.NewAnyWithValue(msgSend2)

	// Build TX with 3 messages
	txBytes := buildTxWithMessagesDemo(
		protoCodec,
		privKey,
		[]*codectypes.Any{sendAny1, delegateAny, sendAny2},
	)
	fmt.Printf("✓ Transaction created with %d bytes and 3 messages\n", len(txBytes))
	fmt.Printf("  - Message 0: MsgSend (9999 ATOM) ← MALICIOUS, will try to skip\n")
	fmt.Printf("  - Message 1: MsgDelegate (777 stake)\n")
	fmt.Printf("  - Message 2: MsgSend (100 ATOM)\n")
	fmt.Println()

	// Locate all 3 messages
	allOffsets, _ := locateMessageOffsetsDemo(txBytes, 3)

	// ATTACK: Only prove messages 1 and 2, skip message 0
	fmt.Println("🔴 ATTACK: Attacker tries to skip Message 0 and only prove Messages 1 & 2")
	fmt.Println()

	delegateFieldKey := byte((1 << 3) | 2)
	delegateFieldValue, _ := extractFieldValueDemo(delegateAny.Value, delegateFieldKey)
	delegateFieldOffset, _ := findFieldOffset(delegateAny.Value, delegateFieldKey, delegateFieldValue)

	sendFieldKey := byte((3 << 3) | 2)
	sendFieldValue, _ := extractFieldValueDemo(sendAny2.Value, sendFieldKey)
	sendFieldOffset, _ := findFieldOffset(sendAny2.Value, sendFieldKey, sendFieldValue)

	attackAssertions := []txsFieldAssertion{
		{
			TypeURL:     delegateAny.TypeUrl,
			FieldKey:    delegateFieldKey,
			FieldValue:  delegateFieldValue,
			FieldOffset: delegateFieldOffset,
			BodyOffset:  allOffsets[1], // Message 1 offset
		},
		{
			TypeURL:     sendAny2.TypeUrl,
			FieldKey:    sendFieldKey,
			FieldValue:  sendFieldValue,
			FieldOffset: sendFieldOffset,
			BodyOffset:  allOffsets[2], // Message 2 offset
		},
	}

	attackConfigs := []txscircuit.MsgConfig{
		{
			MsgTypeLen:    len(delegateAny.TypeUrl),
			FieldValueLen: len(delegateFieldValue),
			MsgValueLen:   len(delegateAny.Value),
		},
		{
			MsgTypeLen:    len(sendAny2.TypeUrl),
			FieldValueLen: len(sendFieldValue),
			MsgValueLen:   len(sendAny2.Value),
		},
	}

	circuit := txscircuit.NewTxsFieldCircuit(len(txBytes), attackConfigs)

	fmt.Println("Compiling circuit...")
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

	fmt.Println("Preparing malicious witness (skipping first message)...")
	witness := prepareTxsWitness(txBytes, attackAssertions, attackConfigs)
	fmt.Println("Witness ready!")

	fmt.Println("Attempting to generate proof with skipped message...")
	fullWitness, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		panic(fmt.Errorf("full witness: %w", err))
	}

	proof, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		fmt.Println("❌ ATTACK FAILED (as expected)!")
		fmt.Printf("   Proof generation failed: %v\n", err)
		fmt.Println()
		fmt.Println("✅ SUCCESS: The circuit constraint successfully prevented message skipping attack!")
		fmt.Println("   The constraint 'msg.BodyOffset must equal cursor' ensures sequential verification.")
		return
	}

	fmt.Println("❌ SECURITY BREACH: Proof generated successfully!")
	fmt.Println("   This means the attacker can skip messages!")
	fmt.Println()

	// Try to verify
	publicWitness, _ := fullWitness.Public()
	if err := groth16.Verify(proof, vk, publicWitness); err != nil {
		fmt.Printf("Proof verification failed: %v\n", err)
	} else {
		fmt.Println("🚨 CRITICAL: Proof verified! Message skipping attack succeeded!")
	}
}

// Run: go run test_message_skipping_attack.go verify-txs-field.go
