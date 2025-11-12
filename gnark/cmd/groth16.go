package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

const (
	storeDir              = "store"
	provingKeyFilename    = "poly_proving.key"
	verifyingKeyFilename  = "poly_verifying.key"
	proofFilename         = "poly_proof.bin"
	publicWitnessFilename = "poly_public.wtns"
)

var groth16Mode string

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

func runGroth16Produce() {
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

	witnessInstance, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		log.Fatalf("witness error: %v", err)
	}

	proof, err := groth16.Prove(cs, pk, witnessInstance)
	if err != nil {
		log.Fatalf("prove error: %v", err)
	}

	publicWitness, err := witnessInstance.Public()
	if err != nil {
		log.Fatalf("public witness error: %v", err)
	}

	if err := ensureStoreDir(); err != nil {
		log.Fatalf("store dir error: %v", err)
	}

	if err := writeToFile(provingKeyPath(), pk); err != nil {
		log.Fatalf("write proving key error: %v", err)
	}
	if err := writeToFile(verifyingKeyPath(), vk); err != nil {
		log.Fatalf("write verifying key error: %v", err)
	}
	if err := writeToFile(proofPath(), proof); err != nil {
		log.Fatalf("write proof error: %v", err)
	}
	if err := writeToFile(publicWitnessPath(), publicWitness); err != nil {
		log.Fatalf("write public witness error: %v", err)
	}

	fmt.Printf("Artifacts written to %s\n", storeDir)
}

func runGroth16Verify() {
	vk, err := readVerifyingKey(verifyingKeyPath())
	if err != nil {
		log.Fatalf("read verifying key error: %v", err)
	}
	proof, err := readProof(proofPath())
	if err != nil {
		log.Fatalf("read proof error: %v", err)
	}
	publicWitness, err := readWitness(publicWitnessPath())
	if err != nil {
		log.Fatalf("read public witness error: %v", err)
	}

	if err := groth16.Verify(proof, vk, publicWitness); err != nil {
		log.Fatalf("stored verification failed: %v", err)
	}

	fmt.Println("Stored Groth16 artifacts verified successfully!")
}

func ensureStoreDir() error {
	return os.MkdirAll(storeDir, 0o755)
}

func provingKeyPath() string {
	return filepath.Join(storeDir, provingKeyFilename)
}

func verifyingKeyPath() string {
	return filepath.Join(storeDir, verifyingKeyFilename)
}

func proofPath() string {
	return filepath.Join(storeDir, proofFilename)
}

func publicWitnessPath() string {
	return filepath.Join(storeDir, publicWitnessFilename)
}

func writeToFile(path string, wt io.WriterTo) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = wt.WriteTo(file)
	return err
}

func readVerifyingKey(path string) (groth16.VerifyingKey, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	vk := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vk.ReadFrom(file); err != nil {
		return nil, err
	}
	return vk, nil
}

func readProof(path string) (groth16.Proof, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	p := groth16.NewProof(ecc.BN254)
	if _, err := p.ReadFrom(file); err != nil {
		return nil, err
	}
	return p, nil
}

func readWitness(path string) (witness.Witness, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	w, err := witness.New(ecc.BN254.ScalarField())
	if err != nil {
		return nil, err
	}
	if _, err := w.ReadFrom(file); err != nil {
		return nil, err
	}
	return w, nil
}
