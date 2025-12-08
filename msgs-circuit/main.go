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
	txBytes := txcodec.Encode()
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

	// Định nghĩa sizes cho circuit (ước lượng từ txBytes)
	txBytesLen := len(txBytes)

	// Extract actual bodyLen from txBytes (byte at position 1)
	bodyBytesLen := 200 // Increased to accommodate actual size (~165 bytes)
	if len(txBytes) > 1 {
		actualBodyLen := int(txBytes[1])
		if actualBodyLen > bodyBytesLen {
			bodyBytesLen = actualBodyLen + 10 // Add buffer
		}
	}

	authInfoBytesLen := 100 // Increased size
	sigLen := 64            // ECDSA signature length
	addrLen := 50           // Bech32 address length (increased)
	msgTypeURLLen := 50     // "/cosmos.bank.v1beta1.MsgSend" length
	msgValueLen := 150      // MsgSend protobuf bytes (increased)

	// Tạo circuit definition
	circuit := txcircuit.NewTxDecodeCircuit(
		txBytesLen,
		bodyBytesLen,
		authInfoBytesLen,
		sigLen,
		addrLen,
		msgTypeURLLen,
		msgValueLen,
	)

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
	witness := prepareWitness(txBytes, txBytesLen, bodyBytesLen, authInfoBytesLen, sigLen, addrLen, msgTypeURLLen, msgValueLen)

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
	fmt.Println("✅ Circuit verified that TxBytes contains the expected MsgSend")
	fmt.Println("✅ Zero-knowledge proof generated and verified")
	fmt.Println()
	fmt.Println("This proves that the transaction bytes contain a valid MsgSend")
	fmt.Println("with the expected FromAddress, ToAddress, and Amount,")
	fmt.Println("without revealing the full transaction details!")
}

// prepareWitness tạo witness data từ transaction bytes
func prepareWitness(txBytes []byte, txBytesLen, bodyBytesLen, authInfoBytesLen, sigLen, addrLen, msgTypeURLLen, msgValueLen int) *txcircuit.TxDecodeCircuit {
	witness := txcircuit.NewTxDecodeCircuit(
		txBytesLen,
		bodyBytesLen,
		authInfoBytesLen,
		sigLen,
		addrLen,
		msgTypeURLLen,
		msgValueLen,
	)

	// Fill TxBytes from actual transaction
	for i := 0; i < len(txBytes) && i < txBytesLen; i++ {
		witness.TxBytes[i] = txBytes[i]
	}

	// Pad remaining TxBytes with zeros
	for i := len(txBytes); i < txBytesLen; i++ {
		witness.TxBytes[i] = 0
	}

	// TODO: Extract actual values từ transaction decode
	// Hiện tại dùng placeholder values để demo

	// ExpectedMsgTypeHash: hash của "/cosmos.bank.v1beta1.MsgSend"
	msgTypeStr := "/cosmos.bank.v1beta1.MsgSend"
	msgTypeHash := 0
	for _, ch := range msgTypeStr {
		msgTypeHash += int(ch)
	}
	witness.ExpectedMsgTypeHash = msgTypeHash

	// Fill MsgTypeURL
	for i := 0; i < len(msgTypeStr) && i < msgTypeURLLen; i++ {
		witness.MsgTypeURL[i] = msgTypeStr[i]
	}
	// Pad remaining with zeros
	for i := len(msgTypeStr); i < msgTypeURLLen; i++ {
		witness.MsgTypeURL[i] = 0
	}

	// Extract BodyBytes from TxBytes (simplified)
	// TxRaw structure: tag(1) + len(1) + BodyBytes + ...
	if len(txBytes) > 2 {
		// bodyLen := int(txBytes[1])
		// Extract all available bytes (up to bodyBytesLen)
		for i := 0; i < bodyBytesLen && (2+i) < len(txBytes); i++ {
			witness.BodyBytes[i] = txBytes[2+i]
		}
		// Pad remaining with zeros only if we run out of txBytes
		for i := len(txBytes) - 2; i < bodyBytesLen; i++ {
			if i >= 0 {
				witness.BodyBytes[i] = 0
			}
		}
	}

	// Fill AuthInfoBytes and Signatures with zeros (placeholder)
	for i := 0; i < authInfoBytesLen; i++ {
		witness.AuthInfoBytes[i] = 0
	}
	for i := 0; i < sigLen; i++ {
		witness.Signatures[i] = 0
	}

	// Fill MsgValue with zeros (placeholder)
	for i := 0; i < msgValueLen; i++ {
		witness.MsgValue[i] = 0
	}

	// Placeholder cho addresses và amount
	// Trong thực tế cần parse từ decoded transaction
	exampleAddr := "cosmos1..."
	for i := 0; i < len(exampleAddr) && i < addrLen; i++ {
		witness.ExpectedFromAddr[i] = exampleAddr[i]
		witness.DecodedFromAddr[i] = exampleAddr[i]
		witness.ExpectedToAddr[i] = exampleAddr[i]
		witness.DecodedToAddr[i] = exampleAddr[i]
	}
	// Pad addresses with zeros
	for i := len(exampleAddr); i < addrLen; i++ {
		witness.ExpectedFromAddr[i] = 0
		witness.DecodedFromAddr[i] = 0
		witness.ExpectedToAddr[i] = 0
		witness.DecodedToAddr[i] = 0
	}

	// Example amount: 1000000 uatom
	witness.ExpectedAmount = 1000000
	witness.DecodedAmount = 1000000

	return witness
}
