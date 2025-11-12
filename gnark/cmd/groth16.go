package cmd

import (
	"fmt"
	"log"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// PolyCircuit enforces f(x, y) = result for
// f(x,y) = 5x^3 - 4x^2y^2 + 13xy^2 + x^2 - 10y.
type PolyCircuit struct {
	X      frontend.Variable
	Y      frontend.Variable
	Result frontend.Variable `gnark:",public"`
}

// Define implements the polynomial constraint.
func (c *PolyCircuit) Define(api frontend.API) error {
	xSquared := api.Mul(c.X, c.X)
	xCubed := api.Mul(xSquared, c.X)
	ySquared := api.Mul(c.Y, c.Y)

	term1 := api.Mul(xCubed, 5)
	term2 := api.Mul(api.Mul(xSquared, ySquared), 4)
	term3 := api.Mul(api.Mul(c.X, ySquared), 13)
	term4 := xSquared
	term5 := api.Mul(c.Y, 10)

	poly := api.Sub(term1, term2)
	poly = api.Add(poly, term3)
	poly = api.Add(poly, term4)
	poly = api.Sub(poly, term5)

	api.AssertIsEqual(poly, c.Result)
	return nil
}

func evaluatePolynomial(x, y int64) int64 {
	return 5*x*x*x - 4*x*x*y*y + 13*x*y*y + x*x - 10*y
}

func runGroth16() {
	var circuit PolyCircuit

	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("compile error: %v", err)
	}

	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		log.Fatalf("setup error: %v", err)
	}

	x := int64(2)
	y := int64(3)
	assignment := PolyCircuit{
		X:      x,
		Y:      y,
		Result: evaluatePolynomial(x, y),
	}

	witness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		log.Fatalf("witness error: %v", err)
	}

	proof, err := groth16.Prove(cs, pk, witness)
	if err != nil {
		log.Fatalf("prove error: %v", err)
	}

	publicWitness, err := witness.Public()
	if err != nil {
		log.Fatalf("public witness error: %v", err)
	}

	if err := groth16.Verify(proof, vk, publicWitness); err != nil {
		log.Fatalf("verification failed: %v", err)
	}

	fmt.Println("Groth16 proof verified successfully!")
}
