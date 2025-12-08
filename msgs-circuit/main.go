package main

import (
	"fmt"

	txcircuit "github.com/DongLieu/msg-circuit/txcircuit"
	txcodec "github.com/DongLieu/msg-circuit/txcodec"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func main() {
	fmt.Println("========== ZK PROOF FOR TX DECODE VERIFICATION ==========")
	fmt.Println()

	// ========================================
	// STEP 1: Tạo transaction từ Encode()
	// ========================================
	fmt.Println("Step 1: Creating transaction using Encode()...")
	txBytes, msgType, fromAddr := txcodec.Encode()
	fmt.Printf("Transaction created: %d bytes\n", len(txBytes))
	fmt.Println()

	// ========================================
	// STEP 2: Decode để lấy thông tin cần verify
	// ========================================
	fmt.Println("Step 2: Decoding transaction to extract message info...")
	txcodec.Decode(txBytes)
	fmt.Println()

	// ========================================
	// STEP 3: Setup circuit với sizes cụ thể
	// ========================================
	fmt.Println("Step 3: Setting up ZK circuit...")

	txBytesLen := len(txBytes)
	msgTypeLen := len(msgType)
	fromAddrLen := len(fromAddr)

	circuit := txcircuit.NewTxDecodeCircuit(txBytesLen, msgTypeLen, fromAddrLen)

	fmt.Printf("Circuit created with TxBytes size: %d\n", txBytesLen)
	fmt.Println()

	// ========================================
	// STEP 4: Compile circuit
	// ========================================
	fmt.Println("Step 4: Compiling circuit to R1CS...")

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		fmt.Printf("Error compiling circuit: %v\n", err)
		return
	}

	fmt.Printf("Circuit compiled successfully!\n")
	fmt.Printf("Number of constraints: %d\n", ccs.GetNbConstraints())
	fmt.Println()

	// ========================================
	// STEP 5: Setup (Generate proving and verifying keys)
	// ========================================
	fmt.Println("Step 5: Generating proving and verifying keys...")

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		fmt.Printf("Error during setup: %v\n", err)
		return
	}

	fmt.Println("Setup completed successfully!")
	fmt.Println()

	// ========================================
	// STEP 6: Prepare witness (assignment)
	// ========================================
	fmt.Println("Step 6: Preparing witness data...")

	// Tạo witness với actual data từ transaction
	witness := prepareWitness(txBytes, msgType, fromAddr, txBytesLen, msgTypeLen, fromAddrLen)

	fmt.Println("Witness prepared!")
	fmt.Println()

	// ========================================
	// STEP 7: Generate proof
	// ========================================
	fmt.Println("Step 7: Generating zero-knowledge proof...")

	fullWitness, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		fmt.Printf("Error creating witness: %v\n", err)
		return
	}

	proof, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		fmt.Printf("Error generating proof: %v\n", err)
		return
	}

	fmt.Println("Proof generated successfully!")
	fmt.Println()

	// ========================================
	// STEP 8: Verify proof
	// ========================================
	fmt.Println("Step 8: Verifying proof...")

	publicWitness, err := fullWitness.Public()
	if err != nil {
		fmt.Printf("Error extracting public witness: %v\n", err)
		return
	}

	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		fmt.Printf("❌ Proof verification FAILED: %v\n", err)
		return
	}

	fmt.Println("✅ Proof verification SUCCEEDED!")
	fmt.Println()

	// ========================================
	// Summary
	// ========================================
	fmt.Println("========== VERIFICATION SUMMARY ==========")
	fmt.Println("✅ Transaction was successfully encoded")
	fmt.Println("✅ Circuit verified the MsgSend type URL")
	fmt.Println("✅ Circuit verified the sender address matches the public input")
	fmt.Println("✅ Zero-knowledge proof generated and verified")
	fmt.Println()
	fmt.Println("This proves that the transaction bytes are bound to the public msgType")
	fmt.Println("and sender address without revealing any other transaction details.")
}

// prepareWitness tạo witness data từ transaction bytes
func prepareWitness(txBytes []byte, msgType string, fromAddr string, txBytesLen, msgTypeLen, addrLen int) *txcircuit.TxDecodeCircuit {
	witness := txcircuit.NewTxDecodeCircuit(txBytesLen, msgTypeLen, addrLen)

	for i := 0; i < txBytesLen; i++ {
		if i < len(txBytes) {
			value := int(txBytes[i])
			witness.TxBytes[i] = value
			witness.PublicTxBytes[i] = value
		} else {
			witness.TxBytes[i] = 0
			witness.PublicTxBytes[i] = 0
		}
	}

	fillBytes := func(dst []frontend.Variable, data []byte) {
		for i := range dst {
			if i < len(data) {
				dst[i] = int(data[i])
			} else {
				dst[i] = 0
			}
		}
	}

	msgTypeBytes := []byte(msgType)
	fillBytes(witness.ExpectedMsgType, msgTypeBytes)

	fromAddrBytes := []byte(fromAddr)
	fillBytes(witness.ExpectedFrom, fromAddrBytes)

	return witness
}
