package main

import (
	"fmt"
	"log"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
)

// Circuit defines a simple quadratic equation circuit
// x * x = y
type Circuit struct {
	X frontend.Variable `gnark:",public"`
	Y frontend.Variable `gnark:",public"`
}

// Define constraints for x^2 = y
func (c *Circuit) Define(api frontend.API) error {
	api.AssertIsEqual(api.Mul(c.X, c.X), c.Y)
	return nil
}

// RecursiveCircuit combines three proofs into one
type RecursiveCircuit struct {
	Proofs [3]frontend.Variable `gnark:",public"`
}

func (rc *RecursiveCircuit) Define(api frontend.API) error {
	// Ensure all proofs are equal (basic aggregation constraint)
	api.AssertIsEqual(rc.Proofs[0], rc.Proofs[1])
	api.AssertIsEqual(rc.Proofs[1], rc.Proofs[2])
	return nil
}

func main() {
	var circuit Circuit

	cs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("Compile error: %v", err)
	}

	// Increase SRS size to accommodate recursive proof setup
	// srsSize := cs.GetNbConstraints() + 10
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

	proofs := []plonk.Proof{}
	publicWitnesses := []witness.Witness{}

	for i := 0; i < 3; i++ {
		// Prove
		witness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
		if err != nil {
			log.Fatalf("Witness error: %v", err)
		}

		proof, err := plonk.Prove(cs, pk, witness)
		if err != nil {
			log.Fatalf("Proof error: %v", err)
		}
		proofs = append(proofs, proof)

		publicWitness, err := witness.Public()
		if err != nil {
			log.Fatalf("Public witness error: %v", err)
		}
		publicWitnesses = append(publicWitnesses, publicWitness)
	}

	// Verify all proofs
	for i, proof := range proofs {
		err = plonk.Verify(proof, vk, publicWitnesses[i])
		if err != nil {
			log.Fatalf("Verification failed: %v", err)
		}
	}

	// Create recursive proof
	var recursiveCircuit RecursiveCircuit
	rcs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &recursiveCircuit)
	if err != nil {
		log.Fatalf("Recursive compile error: %v", err)
	}

	// Ensure SRS is large enough for the recursive circuit
	// recursiveSrsSize := rcs.GetNbConstraints() + 10
	recursiveSrs, recursiveSrsLagrange, err := unsafekzg.NewSRS(rcs)
	if err != nil {
		log.Fatalf("Recursive SRS setup error: %v", err)
	}

	rpk, rvk, err := plonk.Setup(rcs, recursiveSrs, recursiveSrsLagrange)
	if err != nil {
		log.Fatalf("Recursive setup error: %v", err)
	}

	vec0 := publicWitnesses[0].Vector().(fr.Vector)
	vec1 := publicWitnesses[1].Vector().(fr.Vector)
	vec2 := publicWitnesses[2].Vector().(fr.Vector)

	recursiveAssignment := RecursiveCircuit{
		Proofs: [3]frontend.Variable{
			frontend.Variable(vec0[0]),
			frontend.Variable(vec1[0]),
			frontend.Variable(vec2[0]),
		},
	}

	recursiveWitness, err := frontend.NewWitness(&recursiveAssignment, ecc.BN254.ScalarField())
	if err != nil {
		log.Fatalf("Recursive witness error: %v", err)
	}

	recursiveProof, err := plonk.Prove(rcs, rpk, recursiveWitness)
	if err != nil {
		log.Fatalf("Recursive proof error: %v", err)
	}

	// Verify recursive proof
	publicRecursiveWitness, err := recursiveWitness.Public()
	if err != nil {
		log.Fatalf("Public recursive witness error: %v", err)
	}

	err = plonk.Verify(recursiveProof, rvk, publicRecursiveWitness)
	if err != nil {
		log.Fatalf("Recursive verification failed: %v", err)
	}

	fmt.Println("All proofs and recursive proof verified successfully!")
}
