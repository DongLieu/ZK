package main

import (
	// "fmt"

	"crypto/sha256"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// CubicCircuit defines a simple circuit
// x**3 + x + 5 == y
type CubicCircuit struct {
	// struct tags on a variable is optional
	// default uses variable name and secret visibility.
	X frontend.Variable `gnark:"x"`
	Y frontend.Variable `gnark:"x"`
	// Y frontend.Variable `gnark:",public"`
	MsgHash frontend.Variable `gnark:"x"`
}

// Define declares the circuit constraints
// x**3 + x + 5 == y
func (circuit *CubicCircuit) Define(api frontend.API) error {
	x3 := api.Mul(circuit.X, circuit.X, circuit.X)
	api.AssertIsEqual(circuit.Y, api.Add(x3, circuit.X, 5))
	return nil
}

func main() {
	// compiles our circuit into a R1CS
	var circuit CubicCircuit
	ccs, _ := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)

	// groth16 zkSNARK: Setup
	pk, vk, _ := groth16.Setup(ccs)

	// witness definition
	assignment := CubicCircuit{X: 3, Y: 35, MsgHash: hashToField("gui cho Alice")}
	witness, _ := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	publicWitness, _ := witness.Public()

	// groth16: Prove & Verify
	proof, _ := groth16.Prove(ccs, pk, witness)

	var circuit2 CubicCircuit2
	ccs2, _ := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit2)

	// groth16 zkSNARK: Setup
	groth16.Setup(ccs2)

	groth16.Verify(proof, vk, publicWitness)

}

// Hash message into a field element
func hashToField(msg string) *big.Int {
	hash := sha256.Sum256([]byte(msg))
	return new(big.Int).SetBytes(hash[:])
}

// CubicCircuit defines a simple circuit
// x**3 + 2*x + 5 == y
type CubicCircuit2 struct {
	// struct tags on a variable is optional
	// default uses variable name and secret visibility.
	X frontend.Variable `gnark:"x"`
	Y frontend.Variable `gnark:"x"`
	// Y frontend.Variable `gnark:",public"`
	MsgHash frontend.Variable `gnark:"x"`
}

// Define declares the circuit constraints
// x**3 + 2*x + 5 == y
func (circuit *CubicCircuit2) Define(api frontend.API) error {
	x3 := api.Mul(circuit.X, circuit.X, circuit.X)
	_2x := api.Mul(2, circuit.X)
	api.AssertIsEqual(circuit.Y, api.Add(x3, _2x, 5))
	return nil
}
