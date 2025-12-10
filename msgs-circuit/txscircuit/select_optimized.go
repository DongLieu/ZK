package txscircuit

import (
	"github.com/consensys/gnark/frontend"
)

// selectByteAtBinaryTree uses binary tree approach for O(log n) constraints
// Instead of checking every position, we use binary decisions to navigate
func selectByteAtBinaryTree(api frontend.API, tx []frontend.Variable, idx frontend.Variable, maxIdx int) frontend.Variable {
	api.AssertIsLessOrEqual(frontend.Variable(0), idx)
	api.AssertIsLessOrEqual(idx, frontend.Variable(maxIdx))

	// Convert index to binary representation
	numBits := bitsFor(maxIdx + 1)
	idxBits := api.ToBinary(idx, numBits)

	// Build binary tree recursively
	return selectRecursive(api, tx, idxBits, 0, maxIdx, 0)
}

// selectRecursive selects element using binary tree
// bitPos: current bit position we're checking (from MSB to LSB)
func selectRecursive(
	api frontend.API,
	tx []frontend.Variable,
	idxBits []frontend.Variable,
	start int,
	end int,
	bitPos int,
) frontend.Variable {
	// Base case: only one element
	if start == end {
		return tx[start]
	}

	// Find midpoint
	mid := (start + end) / 2

	// Determine which bit to check (MSB first)
	numBits := len(idxBits)
	currentBit := idxBits[numBits-1-bitPos]

	// If bit = 0 → left subtree [start, mid]
	// If bit = 1 → right subtree [mid+1, end]
	leftValue := frontend.Variable(0)
	rightValue := frontend.Variable(0)

	if start <= mid {
		leftValue = selectRecursive(api, tx, idxBits, start, mid, bitPos+1)
	}
	if mid+1 <= end {
		rightValue = selectRecursive(api, tx, idxBits, mid+1, end, bitPos+1)
	}

	// Select based on current bit
	// result = (1 - bit) * left + bit * right
	notBit := api.Sub(1, currentBit)
	result := api.Add(
		api.Mul(notBit, leftValue),
		api.Mul(currentBit, rightValue),
	)

	return result
}

// selectByteAtLookupTable uses lookup table approach
// Good for small arrays (< 256 elements)
func selectByteAtLookupTable(api frontend.API, tx []frontend.Variable, idx frontend.Variable) frontend.Variable {
	if len(tx) == 0 {
		panic("empty array")
	}

	// For small arrays, use direct lookup with multiplexer
	// This is optimal for arrays up to ~16 elements
	if len(tx) <= 16 {
		return selectByteAtLinear(api, tx, idx, len(tx)-1)
	}

	// For larger arrays, use binary tree
	return selectByteAtBinaryTree(api, tx, idx, len(tx)-1)
}

// selectByteAtLinear is the original O(n) implementation
// Kept for comparison and small arrays
func selectByteAtLinear(api frontend.API, tx []frontend.Variable, idx frontend.Variable, maxIdx int) frontend.Variable {
	api.AssertIsLessOrEqual(frontend.Variable(0), idx)
	api.AssertIsLessOrEqual(idx, frontend.Variable(maxIdx))

	result := frontend.Variable(0)
	for pos := 0; pos <= maxIdx; pos++ {
		isPos := api.IsZero(api.Sub(idx, frontend.Variable(pos)))
		result = api.Add(result, api.Mul(isPos, tx[pos]))
	}
	return result
}

// selectByteAtChunked divides array into chunks for better performance
// Hybrid approach: O(n/k + log k) where k is chunk size
func selectByteAtChunked(api frontend.API, tx []frontend.Variable, idx frontend.Variable, maxIdx int, chunkSize int) frontend.Variable {
	api.AssertIsLessOrEqual(frontend.Variable(0), idx)
	api.AssertIsLessOrEqual(idx, frontend.Variable(maxIdx))

	if chunkSize <= 0 {
		chunkSize = 16 // Default chunk size
	}

	numChunks := (maxIdx + chunkSize) / chunkSize

	// Determine which chunk idx belongs to
	chunkResults := make([]frontend.Variable, numChunks)

	for chunkIdx := 0; chunkIdx < numChunks; chunkIdx++ {
		chunkStart := chunkIdx * chunkSize
		chunkEnd := chunkStart + chunkSize - 1
		if chunkEnd > maxIdx {
			chunkEnd = maxIdx
		}

		// Select within this chunk (linear search)
		chunkResult := frontend.Variable(0)
		for pos := chunkStart; pos <= chunkEnd; pos++ {
			isPos := api.IsZero(api.Sub(idx, frontend.Variable(pos)))
			chunkResult = api.Add(chunkResult, api.Mul(isPos, tx[pos]))
		}

		chunkResults[chunkIdx] = chunkResult
	}

	// Select the correct chunk result using binary tree
	idxBits := api.ToBinary(idx, bitsFor(maxIdx+1))
	return selectRecursive(api, chunkResults, idxBits, 0, numChunks-1, 0)
}
