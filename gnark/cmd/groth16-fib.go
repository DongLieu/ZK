package cmd

import (
	"fmt"
	"log"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

const maxFibSteps = 32

// FibCircuit proves Result == fib(N) with fib(0)=0, fib(1)=1 and fib(n)=fib(n-1)+fib(n-2).
// N is kept private while Result is public.
type FibCircuit struct {
	N      frontend.Variable
	Result frontend.Variable `gnark:",public"`
}

// Define enforces the Fibonacci recurrence up to maxFibSteps and binds Result to fib(N).
func (c *FibCircuit) Define(api frontend.API) error {
	api.AssertIsLessOrEqual(c.N, maxFibSteps)

	prev0 := frontend.Variable(0) // fib(0)
	prev1 := frontend.Variable(1) // fib(1)

	// Default to fib(0) when N == 0, fib(1) otherwise until loop overwrites for N >= 2.
	isZero := api.IsZero(c.N)
	result := api.Select(isZero, prev0, prev1)

	for i := 2; i <= maxFibSteps; i++ {
		next := api.Add(prev0, prev1)
		isNi := api.IsZero(api.Sub(c.N, i))
		result = api.Select(isNi, next, result)
		prev0 = prev1
		prev1 = next
	}

	api.AssertIsEqual(result, c.Result)
	return nil
}

func evaluateFib(n int64) int64 {
	if n == 0 {
		return 0
	}
	if n == 1 {
		return 1
	}

	prev0, prev1 := int64(0), int64(1)
	for i := int64(2); i <= n; i++ {
		prev0, prev1 = prev1, prev0+prev1
	}
	return prev1
}

func runGroth16_fib() {
	var circuit FibCircuit

	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("compile error: %v", err)
	}

	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		log.Fatalf("setup error: %v", err)
	}

	n := int64(10)
	assignment := FibCircuit{
		N:      n,
		Result: evaluateFib(n),
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

	fmt.Println("Groth16 Fibonacci proof verified successfully!")
}
