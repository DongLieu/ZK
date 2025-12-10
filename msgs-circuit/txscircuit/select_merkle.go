package txscircuit

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// MerkleProof represents a Merkle proof for array element
type MerkleProof struct {
	Value      frontend.Variable
	Index      int
	Siblings   []frontend.Variable // Sibling hashes along the path
	Directions []int               // 0 = left, 1 = right
}

// selectByteAtMerkle uses Merkle tree for O(log n) verification
// This is the most efficient approach for very large arrays (>1000 elements)
//
// Instead of storing entire array in circuit, we:
// 1. Build Merkle tree of array off-circuit
// 2. Store only root hash as public input
// 3. Provide Merkle proof for the selected element
//
// Constraints: ~log(n) instead of O(n)
func selectByteAtMerkle(
	api frontend.API,
	merkleRoot frontend.Variable,
	proof MerkleProof,
	idx frontend.Variable,
) frontend.Variable {
	// Verify index matches proof
	api.AssertIsEqual(idx, frontend.Variable(proof.Index))

	// Compute Merkle path
	hasher, err := mimc.NewMiMC(api)
	if err != nil {
		panic(err)
	}

	currentHash := proof.Value

	for i := 0; i < len(proof.Siblings); i++ {
		sibling := proof.Siblings[i]
		direction := proof.Directions[i]

		// Hash current with sibling
		hasher.Reset()

		if direction == 0 {
			// Current is on left, sibling on right
			hasher.Write(currentHash)
			hasher.Write(sibling)
		} else {
			// Current is on right, sibling on left
			hasher.Write(sibling)
			hasher.Write(currentHash)
		}

		currentHash = hasher.Sum()
	}

	// Verify computed root matches expected root
	api.AssertIsEqual(currentHash, merkleRoot)

	return proof.Value
}

// MerkleArrayCommitment represents commitment to an array
type MerkleArrayCommitment struct {
	Root frontend.Variable `gnark:",public"`
}

// VerifiedSelect verifies and returns array[idx] using Merkle proof
func (m *MerkleArrayCommitment) VerifiedSelect(
	api frontend.API,
	idx frontend.Variable,
	proof MerkleProof,
) frontend.Variable {
	return selectByteAtMerkle(api, m.Root, proof, idx)
}

// BuildMerkleTree builds Merkle tree for array (off-circuit helper)
// This would be used in witness preparation, not in circuit
/*
func BuildMerkleTree(data []byte) (root []byte, tree [][]byte) {
	n := len(data)

	// Build leaf level
	leaves := make([][]byte, n)
	for i := 0; i < n; i++ {
		hasher := mimc.NewMiMC()
		hasher.Write([]byte{data[i]})
		leaves[i] = hasher.Sum(nil)
	}

	// Build tree level by level
	tree = append(tree, leaves)
	currentLevel := leaves

	for len(currentLevel) > 1 {
		nextLevel := make([][]byte, (len(currentLevel)+1)/2)
		for i := 0; i < len(nextLevel); i++ {
			left := currentLevel[i*2]
			right := left // If odd number, duplicate last
			if i*2+1 < len(currentLevel) {
				right = currentLevel[i*2+1]
			}

			hasher := mimc.NewMiMC()
			hasher.Write(left)
			hasher.Write(right)
			nextLevel[i] = hasher.Sum(nil)
		}
		tree = append(tree, nextLevel)
		currentLevel = nextLevel
	}

	root = currentLevel[0]
	return root, tree
}
*/

// Example usage in TxsFieldCircuit with Merkle commitment
type TxsFieldCircuitMerkle struct {
	TxMerkleRoot  frontend.Variable `gnark:",public"` // Merkle root of tx bytes
	Msgs          []MsgAssertion
	TxBytesProofs []MerkleProof `gnark:",secret"` // Proofs for accessed bytes
}

// This would dramatically reduce constraints:
// - Original: O(n * m) where n=txLen, m=number of accesses
// - Merkle: O(m * log n)
// - For txLen=1000, m=100: 100,000 → 1,000 constraints (100x reduction!)
