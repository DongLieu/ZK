package main

// import (
// 	"bytes"
// 	"fmt"

// 	txcircuit "github.com/DongLieu/msg-circuit/txcircuit"
// 	txcodec "github.com/DongLieu/msg-circuit/txcodec"

// 	"github.com/consensys/gnark-crypto/ecc"
// 	"github.com/consensys/gnark/backend/groth16"
// 	"github.com/consensys/gnark/frontend"
// 	"github.com/consensys/gnark/frontend/cs/r1cs"
// )

// func tx_field() {
// 	fmt.Println("========== ZK PROOF FOR TX FIELD VERIFICATION ==========")
// 	fmt.Println()

// 	// ========================================
// 	// STEP 1: Tạo transaction từ Encode()
// 	// ========================================
// 	fmt.Println("Step 1: Creating transaction using Encode()...")
// 	txBytes, msgType, fromAddr, _, _ := txcodec.Encode()
// 	fmt.Printf("Transaction created: %d bytes\n", len(txBytes))
// 	fmt.Println()

// 	// ========================================
// 	// STEP 2: Decode (log) và lấy thông tin field target
// 	// ========================================
// 	fmt.Println("Step 2: Decoding transaction to inspect message info...")
// 	txcodec.Decode(txBytes)

// 	anyMsg, err := txcodec.ExtractFirstMessage(txBytes)
// 	if err != nil {
// 		panic(fmt.Errorf("failed to extract first message: %w", err))
// 	}

// 	fieldKey := byte(0x0a) // ví dụ: field number 1 (from_address) với wire-type 2
// 	fieldValue := []byte(fromAddr)

// 	fieldOffset, err := findFieldOffset(anyMsg.Value, fieldKey, fieldValue)
// 	if err != nil {
// 		panic(err)
// 	}

// 	fmt.Println("Target field info:")
// 	fmt.Printf("  TypeURL: %s\n", anyMsg.TypeUrl)
// 	fmt.Printf("  Field key: 0x%x | value length: %d bytes | offset: %d\n", fieldKey, len(fieldValue), fieldOffset)
// 	fmt.Println()

// 	// ========================================
// 	// STEP 3: Setup circuit với sizes cụ thể
// 	// ========================================
// 	fmt.Println("Step 3: Setting up ZK circuit...")

// 	txBytesLen := len(txBytes)
// 	msgTypeLen := len(msgType)
// 	fieldValueLen := len(fieldValue)
// 	msgValueLen := len(anyMsg.Value)

// 	circuit := txcircuit.NewTxFieldCircuit(txBytesLen, msgTypeLen, fieldValueLen, msgValueLen)

// 	fmt.Printf("Circuit created with TxBytes size: %d\n", txBytesLen)
// 	fmt.Println()

// 	// ========================================
// 	// STEP 4: Compile circuit
// 	// ========================================
// 	fmt.Println("Step 4: Compiling circuit to R1CS...")

// 	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
// 	if err != nil {
// 		fmt.Printf("Error compiling circuit: %v\n", err)
// 		return
// 	}

// 	fmt.Printf("Circuit compiled successfully!\n")
// 	fmt.Printf("Number of constraints: %d\n", ccs.GetNbConstraints())
// 	fmt.Println()

// 	// ========================================
// 	// STEP 5: Setup (Generate proving and verifying keys)
// 	// ========================================
// 	fmt.Println("Step 5: Generating proving and verifying keys...")

// 	pk, vk, err := groth16.Setup(ccs)
// 	if err != nil {
// 		fmt.Printf("Error during setup: %v\n", err)
// 		return
// 	}

// 	fmt.Println("Setup completed successfully!")
// 	fmt.Println()

// 	// ========================================
// 	// STEP 6: Prepare witness (assignment)
// 	// ========================================
// 	fmt.Println("Step 6: Preparing witness data...")

// 	witness := prepareWitness(
// 		txBytes,
// 		msgType,
// 		fieldKey,
// 		fieldValue,
// 		fieldOffset,
// 		txBytesLen,
// 		msgTypeLen,
// 		fieldValueLen,
// 		msgValueLen,
// 	)

// 	fmt.Println("Witness prepared!")
// 	fmt.Println()

// 	// ========================================
// 	// STEP 7: Generate proof
// 	// ========================================
// 	fmt.Println("Step 7: Generating zero-knowledge proof...")

// 	fullWitness, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
// 	if err != nil {
// 		fmt.Printf("Error creating witness: %v\n", err)
// 		return
// 	}

// 	proof, err := groth16.Prove(ccs, pk, fullWitness)
// 	if err != nil {
// 		fmt.Printf("Error generating proof: %v\n", err)
// 		return
// 	}

// 	fmt.Println("Proof generated successfully!")
// 	fmt.Println()

// 	// ========================================
// 	// STEP 8: Verify proof
// 	// ========================================
// 	fmt.Println("Step 8: Verifying proof...")

// 	publicWitness, err := fullWitness.Public()
// 	if err != nil {
// 		fmt.Printf("Error extracting public witness: %v\n", err)
// 		return
// 	}

// 	err = groth16.Verify(proof, vk, publicWitness)
// 	if err != nil {
// 		fmt.Printf("❌ Proof verification FAILED: %v\n", err)
// 		return
// 	}

// 	fmt.Println("✅ Proof verification SUCCEEDED!")
// 	fmt.Println()

// 	// ========================================
// 	// Summary
// 	// ========================================
// 	fmt.Println("========== VERIFICATION SUMMARY ==========")
// 	fmt.Println("✅ Transaction was successfully encoded")
// 	fmt.Println("✅ Circuit keeps the Msg TypeURL secret")
// 	fmt.Println("✅ Circuit proved the tx encodes the chosen public field (key/value)")
// 	fmt.Println("✅ Zero-knowledge proof generated and verified")
// 	fmt.Println()
// 	fmt.Println("This binds the hidden msgType to any Cosmos field you expose publicly (depositor, delegator, amount, ...).")
// }

// func prepareWitness(
// 	txBytes []byte,
// 	msgType string,
// 	fieldKey byte,
// 	fieldValue []byte,
// 	fieldOffset int,
// 	txBytesLen,
// 	msgTypeLen,
// 	fieldValueLen,
// 	msgValueLen int,
// ) *txcircuit.TxFieldCircuit {
// 	witness := txcircuit.NewTxFieldCircuit(txBytesLen, msgTypeLen, fieldValueLen, msgValueLen)

// 	for i := 0; i < txBytesLen; i++ {
// 		if i < len(txBytes) {
// 			value := int(txBytes[i])
// 			witness.TxBytes[i] = value
// 			witness.PublicTxBytes[i] = value
// 		} else {
// 			witness.TxBytes[i] = 0
// 			witness.PublicTxBytes[i] = 0
// 		}
// 	}

// 	fillBytes := func(dst []frontend.Variable, data []byte) {
// 		for i := range dst {
// 			if i < len(data) {
// 				dst[i] = int(data[i])
// 			} else {
// 				dst[i] = 0
// 			}
// 		}
// 	}

// 	fillBytes(witness.MsgType, []byte(msgType))
// 	fillBytes(witness.Field.Value, fieldValue)

// 	witness.Field.Key = int(fieldKey)
// 	witness.FieldOffset = fieldOffset

// 	return witness
// }

// // func findFieldOffset(msgValue []byte, fieldKey byte, fieldValue []byte) (int, error) {
// // 	needle := []byte{fieldKey}
// // 	needle = append(needle, encodeVarint(len(fieldValue))...)
// // 	needle = append(needle, fieldValue...)

// // 	offset := bytes.Index(msgValue, needle)
// // 	if offset < 0 {
// // 		return 0, fmt.Errorf("field key 0x%x with provided value not found in message", fieldKey)
// // 	}

// // 	return offset, nil
// // }

// func encodeVarint(length int) []byte {
// 	if length == 0 {
// 		return []byte{0}
// 	}

// 	var out []byte
// 	value := length
// 	for value > 0 {
// 		b := byte(value & 0x7f)
// 		value >>= 7
// 		if value > 0 {
// 			b |= 0x80
// 		}
// 		out = append(out, b)
// 	}

// 	return out
// }
