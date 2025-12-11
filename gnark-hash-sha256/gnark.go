package main

import (
	"crypto/sha256"
	"fmt"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/hash/sha2"
	"github.com/consensys/gnark/std/math/uints"
)

// SHA256Circuit defines the circuit structure
// Note: Gnark's sha2.New() uses SHA256, not SHA512
// output = SHA256(SHA256(SHA256(a) + b) + c)
type SHA512Circuit struct {
	// Private inputs (witness) - as bytes
	A []uints.U8 `gnark:",secret"`
	C []uints.U8 `gnark:",secret"`

	// Public inputs - as bytes
	B      []uints.U8 `gnark:",public"`
	Output []uints.U8 `gnark:",public"` // 32 bytes for SHA256
}

// Define declares the circuit constraints
// Computes: output = SHA256(SHA256(SHA256(a) + b) + c)
func (circuit *SHA512Circuit) Define(api frontend.API) error {
	// Step 1: hash1 = SHA512(a)
	h1, err := sha2.New(api)
	if err != nil {
		return err
	}

	// Write input 'a' to hasher
	h1.Write(circuit.A)
	hash1 := h1.Sum()

	// Step 2: Concatenate hash1 + b
	hash1PlusB := append(hash1, circuit.B...)

	// Step 3: hash2 = SHA512(hash1 + b)
	h2, err := sha2.New(api)
	if err != nil {
		return err
	}
	h2.Write(hash1PlusB)
	hash2 := h2.Sum()

	// Step 4: Concatenate hash2 + c
	hash2PlusC := append(hash2, circuit.C...)

	// Step 5: output = SHA512(hash2 + c)
	h3, err := sha2.New(api)
	if err != nil {
		return err
	}
	h3.Write(hash2PlusC)
	finalHash := h3.Sum()

	// Step 6: Constrain output to match computed hash
	if len(circuit.Output) != len(finalHash) {
		return fmt.Errorf("output length mismatch: expected %d, got %d", len(finalHash), len(circuit.Output))
	}

	for i := 0; i < len(finalHash); i++ {
		api.AssertIsEqual(circuit.Output[i].Val, finalHash[i].Val)
	}

	return nil
}

func main() {
	demoSingleHash()
}

func demo3Hash() {
	// Example inputs (in bytes)
	a := []byte("secret_value_a_123456789012345678901234567890123456789012345")
	b := []byte("public_value_b_123456789012345678901234567890123456789012345")
	c := []byte("secret_value_c_123456789012345678901234567890123456789012345")

	// Pad to 64 bytes if needed
	aPadded := make([]byte, 64)
	bPadded := make([]byte, 64)
	cPadded := make([]byte, 64)
	copy(aPadded, a)
	copy(bPadded, b)
	copy(cPadded, c)

	// Compute expected output off-circuit using SHA256 (matching Gnark's sha2)
	// Step 1: hash1 = SHA256(a)
	h1 := sha256.Sum256(aPadded)

	// Step 2: hash2 = SHA256(hash1 || b)
	h2Input := append(h1[:], bPadded...)
	h2 := sha256.Sum256(h2Input)

	// Step 3: output = SHA256(hash2 || c)
	h3Input := append(h2[:], cPadded...)
	h3 := sha256.Sum256(h3Input)
	expectedOutput := h3[:]

	fmt.Printf("Expected Output (SHA256): %x\n", expectedOutput)
	fmt.Printf("Output length: %d bytes\n", len(expectedOutput))

	// Step 1: Compile the circuit
	fmt.Println("\n=== Compiling Circuit ===")

	// Create circuit with proper sizes
	// Gnark's sha2.New() returns SHA256 (32 bytes output)
	var circuit SHA512Circuit
	circuit.A = make([]uints.U8, 64)
	circuit.B = make([]uints.U8, 64)
	circuit.C = make([]uints.U8, 64)
	circuit.Output = make([]uints.U8, 32) // SHA256 output is 32 bytes

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Number of constraints: %d\n", ccs.GetNbConstraints())

	// Step 2: Generate proving and verifying keys (Trusted Setup)
	fmt.Println("\n=== Trusted Setup ===")
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}
	fmt.Println("Setup completed successfully")

	// Step 3: Prepare witness (assignment)
	var assignment SHA512Circuit

	// Convert bytes to uints.U8
	assignment.A = make([]uints.U8, 64)
	assignment.B = make([]uints.U8, 64)
	assignment.C = make([]uints.U8, 64)
	assignment.Output = make([]uints.U8, 32) // SHA256 output is 32 bytes

	for i := 0; i < 64; i++ {
		assignment.A[i] = uints.NewU8(aPadded[i])
		assignment.B[i] = uints.NewU8(bPadded[i])
		assignment.C[i] = uints.NewU8(cPadded[i])
	}

	// Assign 32 bytes for output (SHA256)
	for i := 0; i < 32; i++ {
		assignment.Output[i] = uints.NewU8(expectedOutput[i])
	}

	// Step 4: Create witness
	fmt.Println("\n=== Creating Witness ===")
	witness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		panic(err)
	}
	fmt.Println("Witness created successfully")

	// Step 5: Generate proof
	fmt.Println("\n=== Generating Proof ===")
	proof, err := groth16.Prove(ccs, pk, witness)
	if err != nil {
		panic(err)
	}
	fmt.Println("Proof generated successfully")

	// Step 6: Extract public witness for verification
	publicWitness, err := witness.Public()
	if err != nil {
		panic(err)
	}

	// Step 7: Verify proof
	fmt.Println("\n=== Verifying Proof ===")
	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		fmt.Printf("Verification failed: %v\n", err)
	} else {
		fmt.Println("✓ Proof verified successfully!")
		fmt.Printf("\nPublic Input B: %s\n", string(bPadded))
		fmt.Printf("Public Output: %x\n", expectedOutput)
	}
}

type SingleSHA256Circuit struct {
	Input  []frontend.Variable `gnark:",secret"`
	Output [32]uints.U8        `gnark:",public"`
	length int
}

func (circuit *SingleSHA256Circuit) Define(api frontend.API) error {
	byteField, err := uints.New[uints.U32](api)
	if err != nil {
		return err
	}
	txBytes := make([]uints.U8, circuit.length)
	for i := 0; i < circuit.length; i++ {
		txBytes[i] = byteField.ByteValueOf(circuit.Input[i])
	}

	hasher, err := sha2.New(api)
	if err != nil {
		return err
	}
	hasher.Write(txBytes)
	digest := hasher.Sum()

	for i := 0; i < len(digest); i++ {
		api.AssertIsEqual(circuit.Output[i].Val, digest[i].Val)
	}
	return nil
}

func demoSingleHash() {
	hex := "0a2d636f736d6f733138717176377272757a663832687479717a6e37673366393337323265786c6d636b7465383267122d636f736d6f73313764326172363373307179766e64327a3965736e7934797930766e7730657878637463326e791a100a057561746f6d120731303030303030"
	secret := []byte(hex)
	expected := sha256.Sum256(secret)

	var circuit SingleSHA256Circuit
	circuit.Input = make([]frontend.Variable, len(secret))
	circuit.length = len(secret)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Single-hash circuit constraints: %d\n", ccs.GetNbConstraints())

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}

	var assignment SingleSHA256Circuit
	assignment.Input = make([]frontend.Variable, len(secret))
	for i := range assignment.Input {
		if i < len(secret) {
			assignment.Input[i] = int(secret[i])
		} else {
			assignment.Input[i] = 0
		}
	}

	for i := 0; i < 32; i++ {
		assignment.Output[i] = uints.NewU8(expected[i])
	}

	witness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		panic(err)
	}
	publicWitness, err := witness.Public()
	if err != nil {
		panic(err)
	}

	proof, err := groth16.Prove(ccs, pk, witness)
	if err != nil {
		panic(err)
	}
	if err := groth16.Verify(proof, vk, publicWitness); err != nil {
		panic(err)
	}
	fmt.Printf("Single hash proof verified! Output: %x\n", expected[:])
}
