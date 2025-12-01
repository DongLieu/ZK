package cmd

import (
	"crypto/sha512"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/consensys/gnark-crypto/ecc"
	fr "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend/groth16"
	groth16bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/backend/witness"
	// "github.com/consensys/gnark/constraint"
	csbn254 "github.com/consensys/gnark/constraint/bn254"
)

// func readConstraintSystem(path string) (constraint.ConstraintSystem, error) {
// 	file, err := os.Open(path)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer file.Close()

// 	cs := groth16.NewCS(ecc.BN254)
// 	if _, err := cs.ReadFrom(file); err != nil {
// 		return nil, err
// 	}
// 	return cs, nil
// }

func main_dump_info() {
	csAny, err := readConstraintSystem("store-ed25519-batch/poly_circuit.r1cs")
	if err != nil {
		log.Fatalf("read cs: %v", err)
	}
	r1cs := csAny.(*csbn254.R1CS)
	fmt.Printf("Commitment info: %+v\n", r1cs.CommitmentInfo)

	proofFile, err := os.Open("store-ed25519-batch/poly_proof.bin")
	if err != nil {
		log.Fatalf("open proof: %v", err)
	}
	defer proofFile.Close()
	proofGeneric := groth16.NewProof(ecc.BN254)
	if _, err := proofGeneric.ReadFrom(proofFile); err != nil {
		log.Fatalf("read proof: %v", err)
	}
	proof := proofGeneric.(*groth16bn254.Proof)
	fmt.Printf("Number of commitments: %d\n", len(proof.Commitments))
	if len(proof.Commitments) == 0 {
		log.Fatalf("no commitments found")
	}
	commitBytes := proof.Commitments[0].Marshal()
	h := sha512.Sum512(commitBytes)
	val := new(big.Int).SetBytes(h[:])
	r := fr.Modulus()
	val.Mod(val, r)
	fmt.Printf("sha512 commitment -> %s\n", val.String())

	wFile, err := os.Open("store-ed25519-batch/poly_public.wtns")
	if err != nil {
		log.Fatalf("open public witness: %v", err)
	}
	defer wFile.Close()
	w, err := witness.New(ecc.BN254.ScalarField())
	if err != nil {
		log.Fatalf("new witness: %v", err)
	}
	if _, err := w.ReadFrom(wFile); err != nil {
		log.Fatalf("read witness: %v", err)
	}
	publicW, err := w.Public()
	if err != nil {
		log.Fatalf("public witness: %v", err)
	}
	vec, ok := publicW.Vector().(fr.Vector)
	if !ok {
		log.Fatalf("unexpected vector type")
	}
	fmt.Printf("public witness length: %d\n", len(vec))
	for i := 0; i < len(vec); i++ {
		bi := new(big.Int)
		vec[i].BigInt(bi)
		fmt.Printf("public[%d] = %s\n", i, bi.String())
	}
}
