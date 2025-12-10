package txscircuit

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
)

// TestCircuit for benchmarking select methods
type SelectBenchmarkCircuit struct {
	Array  []frontend.Variable
	Index  frontend.Variable
	Result frontend.Variable `gnark:",public"`
}

func (circuit *SelectBenchmarkCircuit) Define(api frontend.API) error {
	// Test the select operation
	selected := selectByteAt(api, circuit.Array, circuit.Index, len(circuit.Array)-1)
	api.AssertIsEqual(selected, circuit.Result)
	return nil
}

type SelectBenchmarkCircuitBinaryTree struct {
	Array  []frontend.Variable
	Index  frontend.Variable
	Result frontend.Variable `gnark:",public"`
}

func (circuit *SelectBenchmarkCircuitBinaryTree) Define(api frontend.API) error {
	selected := selectByteAtBinaryTree(api, circuit.Array, circuit.Index, len(circuit.Array)-1)
	api.AssertIsEqual(selected, circuit.Result)
	return nil
}

type SelectBenchmarkCircuitChunked struct {
	Array  []frontend.Variable
	Index  frontend.Variable
	Result frontend.Variable `gnark:",public"`
}

func (circuit *SelectBenchmarkCircuitChunked) Define(api frontend.API) error {
	selected := selectByteAtChunked(api, circuit.Array, circuit.Index, len(circuit.Array)-1, 16)
	api.AssertIsEqual(selected, circuit.Result)
	return nil
}

func BenchmarkSelectLinear100(b *testing.B) {
	benchmarkSelect(b, 100, false, false)
}

func BenchmarkSelectBinaryTree100(b *testing.B) {
	benchmarkSelect(b, 100, true, false)
}

func BenchmarkSelectChunked100(b *testing.B) {
	benchmarkSelect(b, 100, false, true)
}

func BenchmarkSelectLinear1000(b *testing.B) {
	benchmarkSelect(b, 1000, false, false)
}

func BenchmarkSelectBinaryTree1000(b *testing.B) {
	benchmarkSelect(b, 1000, true, false)
}

func BenchmarkSelectChunked1000(b *testing.B) {
	benchmarkSelect(b, 1000, false, true)
}

func benchmarkSelect(b *testing.B, arraySize int, useBinaryTree bool, useChunked bool) {
	// Create test array
	array := make([]frontend.Variable, arraySize)
	for i := 0; i < arraySize; i++ {
		array[i] = i
	}

	// Select middle element
	// index := arraySize / 2

	var circuit frontend.Circuit
	if useBinaryTree {
		circuit = &SelectBenchmarkCircuitBinaryTree{
			Array: array,
		}
	} else if useChunked {
		circuit = &SelectBenchmarkCircuitChunked{
			Array: array,
		}
	} else {
		circuit = &SelectBenchmarkCircuit{
			Array: array,
		}
	}

	// Compile and count constraints
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportMetric(float64(ccs.GetNbConstraints()), "constraints")
	b.ReportMetric(float64(ccs.GetNbConstraints())/float64(arraySize), "constraints/element")
}

func TestSelectMethods(t *testing.T) {
	sizes := []int{10, 100, 500}

	for _, size := range sizes {
		array := make([]frontend.Variable, size)
		for i := 0; i < size; i++ {
			array[i] = i + 1 // 1-indexed
		}

		// Test linear
		{
			circuit := &SelectBenchmarkCircuit{Array: array}
			witness := &SelectBenchmarkCircuit{
				Array:  array,
				Index:  25,
				Result: 26, // array[25] = 26
			}

			assert := test.NewAssert(t)
			err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
			assert.NoError(err, "Linear select failed for size %d", size)
		}

		// Test binary tree
		{
			circuit := &SelectBenchmarkCircuitBinaryTree{Array: array}
			witness := &SelectBenchmarkCircuitBinaryTree{
				Array:  array,
				Index:  25,
				Result: 26,
			}

			assert := test.NewAssert(t)
			err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
			assert.NoError(err, "Binary tree select failed for size %d", size)
		}

		// Test chunked
		{
			circuit := &SelectBenchmarkCircuitChunked{Array: array}
			witness := &SelectBenchmarkCircuitChunked{
				Array:  array,
				Index:  25,
				Result: 26,
			}

			assert := test.NewAssert(t)
			err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
			assert.NoError(err, "Chunked select failed for size %d", size)
		}
	}
}

// Constraint comparison test
func TestConstraintComparison(t *testing.T) {
	type result struct {
		method      string
		size        int
		constraints int
	}

	var results []result

	sizes := []int{10, 50, 100, 500, 1000}

	for _, size := range sizes {
		array := make([]frontend.Variable, size)

		// Linear
		{
			circuit := &SelectBenchmarkCircuit{Array: array}
			ccs, _ := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
			results = append(results, result{"Linear", size, ccs.GetNbConstraints()})
		}

		// Binary tree
		{
			circuit := &SelectBenchmarkCircuitBinaryTree{Array: array}
			ccs, _ := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
			results = append(results, result{"BinaryTree", size, ccs.GetNbConstraints()})
		}

		// Chunked
		{
			circuit := &SelectBenchmarkCircuitChunked{Array: array}
			ccs, _ := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
			results = append(results, result{"Chunked", size, ccs.GetNbConstraints()})
		}
	}

	// Print results
	t.Log("Constraint Comparison:")
	t.Log("Size\tLinear\tBinaryTree\tChunked\tImprovement")
	for i := 0; i < len(results); i += 3 {
		linear := results[i].constraints
		binary := results[i+1].constraints
		chunked := results[i+2].constraints
		improvement := float64(linear) / float64(binary)

		t.Logf("%d\t%d\t%d\t\t%d\t%.1fx",
			results[i].size,
			linear,
			binary,
			chunked,
			improvement,
		)
	}
}
