package main

import (
	"fmt"
	"log"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
)

// x * x = y
type Circuit struct {
	X frontend.Variable `gnark:",public"`
	Y frontend.Variable `gnark:",public"`
}

// x^2 = y
func (c *Circuit) Define(api frontend.API) error {
	api.AssertIsEqual(api.Mul(c.X, c.X), c.Y)
	return nil
}

func main() {
	var circuit Circuit

	cs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("Compile error: %v", err)
	}

	srs, srsLagrange, err := unsafekzg.NewSRS(cs)
	if err != nil {
		log.Fatalf("Setup error: %v", err)
	}
	pk, vk, err := plonk.Setup(cs, srs, srsLagrange)
	if err != nil {
		log.Fatalf("Setup error: %v", err)
	}

	assignment := Circuit{
		X: 3,
		Y: 9, // 3 * 3 = 9
	}

	// Prove
	witness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		log.Fatalf("Witness error: %v", err)
	}

	proof, err := plonk.Prove(cs, pk, witness)
	if err != nil {
		log.Fatalf("Proof error: %v", err)
	}

	// Verify
	publicWitness, err := witness.Public()
	if err != nil {
		log.Fatalf("Public witness error: %v", err)
	}

	err = plonk.Verify(proof, vk, publicWitness)
	if err != nil {
		log.Fatalf("Verification failed: %v", err)
	}

	fmt.Println("Proof verified successfully!")
}
