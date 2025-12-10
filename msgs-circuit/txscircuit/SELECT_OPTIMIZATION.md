# SelectByteAt Optimization: O(n) → O(log n)

## Vấn đề hiện tại

`selectByteAt()` hiện tại có **O(n) constraints**:

```go
func selectByteAt(api frontend.API, tx []frontend.Variable, idx frontend.Variable, maxIdx int) frontend.Variable {
    result := frontend.Variable(0)
    for pos := 0; pos <= maxIdx; pos++ {
        isPos := api.IsZero(api.Sub(idx, frontend.Variable(pos)))
        result = api.Add(result, api.Mul(isPos, tx[pos]))
    }
    return result
}
```

**Constraints:** ~3n (IsZero + Mul + Add cho mỗi position)

## Phương pháp cải tiến

### 1. Binary Tree Selection - O(log n)

#### Cách hoạt động

```
Array: [0, 1, 2, 3, 4, 5, 6, 7]
Index: 5 (binary: 101)

Binary tree:
              [0-7]
            /       \
        [0-3]       [4-7]
        /   \       /   \
    [0-1] [2-3] [4-5] [6-7]
     / \   / \   / \   / \
    0  1  2  3  4  5  6  7

Path for idx=5 (101):
- Bit 2 (MSB) = 1 → right [4-7]
- Bit 1       = 0 → left [4-5]
- Bit 0 (LSB) = 1 → right → 5 ✓
```

#### Implementation

```go
func selectByteAtBinaryTree(api frontend.API, tx []frontend.Variable, idx frontend.Variable, maxIdx int) frontend.Variable {
    numBits := bitsFor(maxIdx + 1)
    idxBits := api.ToBinary(idx, numBits)
    return selectRecursive(api, tx, idxBits, 0, maxIdx, 0)
}

func selectRecursive(api frontend.API, tx []frontend.Variable, idxBits []frontend.Variable, start int, end int, bitPos int) frontend.Variable {
    if start == end {
        return tx[start]
    }

    mid := (start + end) / 2
    currentBit := idxBits[len(idxBits)-1-bitPos]

    leftValue := selectRecursive(api, tx, idxBits, start, mid, bitPos+1)
    rightValue := selectRecursive(api, tx, idxBits, mid+1, end, bitPos+1)

    // result = (1 - bit) * left + bit * right
    notBit := api.Sub(1, currentBit)
    return api.Add(
        api.Mul(notBit, leftValue),
        api.Mul(currentBit, rightValue),
    )
}
```

#### Constraints Analysis

**Per selection:**
- ToBinary: ~log(n) constraints
- Recursive tree: 2 * log(n) constraints (2 Muls per level)
- **Total: ~3 * log(n) constraints**

**Ví dụ với n=1000:**
- Original: ~3000 constraints
- Binary tree: ~30 constraints
- **Improvement: 100x reduction!**

### 2. Chunked Selection - O(n/k + log k)

#### Cách hoạt động

Chia array thành chunks, search trong chunk (linear), chọn chunk (binary tree):

```
Array (size 1000) → 10 chunks (size 100 each)
1. Linear search in each chunk: 100 constraints
2. Binary tree select chunk: ~3 constraints
Total: ~103 constraints (vs 3000 original)
```

#### Implementation

```go
func selectByteAtChunked(api frontend.API, tx []frontend.Variable, idx frontend.Variable, maxIdx int, chunkSize int) frontend.Variable {
    numChunks := (maxIdx + chunkSize) / chunkSize
    chunkResults := make([]frontend.Variable, numChunks)

    // Linear search within each chunk
    for chunkIdx := 0; chunkIdx < numChunks; chunkIdx++ {
        chunkStart := chunkIdx * chunkSize
        chunkEnd := min(chunkStart + chunkSize - 1, maxIdx)

        chunkResult := frontend.Variable(0)
        for pos := chunkStart; pos <= chunkEnd; pos++ {
            isPos := api.IsZero(api.Sub(idx, frontend.Variable(pos)))
            chunkResult = api.Add(chunkResult, api.Mul(isPos, tx[pos]))
        }
        chunkResults[chunkIdx] = chunkResult
    }

    // Binary tree to select correct chunk
    idxBits := api.ToBinary(idx, bitsFor(maxIdx+1))
    return selectRecursive(api, chunkResults, idxBits, 0, numChunks-1, 0)
}
```

#### Optimal Chunk Size

For array size n:
- Too small chunks: More overhead from tree selection
- Too large chunks: More linear search

**Optimal: chunkSize ≈ sqrt(n)**

| Array Size | Optimal Chunk | Constraints | Original | Improvement |
|------------|---------------|-------------|----------|-------------|
| 100        | 10            | ~30         | 300      | 10x         |
| 1000       | 32            | ~96         | 3000     | 31x         |
| 10000      | 100           | ~300        | 30000    | 100x        |

### 3. Merkle Tree Approach - O(log n)

#### Cách hoạt động

Thay vì store toàn bộ array trong circuit, chỉ store Merkle root:

```
Array: [a, b, c, d, e, f, g, h]

Build Merkle tree:
              Root
            /      \
       H(ab,cd)   H(ef,gh)
        /  \       /   \
     H(ab) H(cd) H(ef) H(gh)
      / \   / \   / \   / \
     a  b  c  d  e  f  g  h

Public input: Root hash
Private input:
  - Index: 5
  - Value: f
  - Proof: [e, H(gh), H(ab,cd)]
```

#### Implementation

```go
type MerkleProof struct {
    Value      frontend.Variable
    Index      int
    Siblings   []frontend.Variable
    Directions []int
}

func selectByteAtMerkle(api frontend.API, merkleRoot frontend.Variable, proof MerkleProof, idx frontend.Variable) frontend.Variable {
    api.AssertIsEqual(idx, frontend.Variable(proof.Index))

    hasher, _ := mimc.NewMiMC(api)
    currentHash := proof.Value

    for i := 0; i < len(proof.Siblings); i++ {
        hasher.Reset()
        if proof.Directions[i] == 0 {
            hasher.Write(currentHash)
            hasher.Write(proof.Siblings[i])
        } else {
            hasher.Write(proof.Siblings[i])
            hasher.Write(currentHash)
        }
        currentHash = hasher.Sum()
    }

    api.AssertIsEqual(currentHash, merkleRoot)
    return proof.Value
}
```

#### Constraints Analysis

**Per selection:**
- Hash operations: ~log(n) hashes
- MiMC hash: ~50 constraints each
- **Total: ~50 * log(n) constraints**

**Ví dụ với n=1000:**
- Original: ~3000 constraints
- Merkle: ~500 constraints
- **Improvement: 6x reduction**

**Bonus:** Public input size giảm từ O(n) → O(1)!

## So sánh tổng quát

### Constraints per selection

| Method          | Complexity | n=100  | n=1000 | n=10000 | Notes              |
|-----------------|------------|--------|--------|---------|-------------------|
| Linear (original)| O(n)      | 300    | 3000   | 30000   | Current           |
| Binary Tree     | O(log n)   | 20     | 30     | 40      | **Best for pure select** |
| Chunked (k=√n)  | O(√n)      | 30     | 96     | 300     | Good balance      |
| Merkle Tree     | O(log n)   | 350    | 500    | 650     | Best for large data |

### Public input size

| Method          | Public Input Size |
|-----------------|-------------------|
| Linear          | O(n) - Full array |
| Binary Tree     | O(n) - Full array |
| Chunked         | O(n) - Full array |
| Merkle Tree     | **O(1) - Just root hash** |

## Khuyến nghị cho TxsFieldCircuit

### Short-term (Easy win): Binary Tree

**Replace `selectByteAt()` với `selectByteAtBinaryTree()`:**

```go
// In circuit.go, replace all calls:
// OLD:
keyByte := selectByteAt(api, tx, fieldStart, maxIdx)

// NEW:
keyByte := selectByteAtBinaryTree(api, tx, fieldStart, maxIdx)
```

**Impact:**
- Constraints reduction: ~100x
- No changes to witness structure
- Drop-in replacement

### Medium-term: Chunked Hybrid

For very large transactions (>10KB):

```go
// Use chunked for large arrays
if len(tx) > 1000 {
    return selectByteAtChunked(api, tx, idx, maxIdx, 32)
} else {
    return selectByteAtBinaryTree(api, tx, idx, maxIdx)
}
```

### Long-term: Merkle Commitment

Redesign circuit to use Merkle root:

```go
type TxsFieldCircuit struct {
    TxMerkleRoot  frontend.Variable `gnark:",public"`  // Just 1 field element!
    Msgs          []MsgAssertion
    ByteProofs    []MerkleProof     `gnark:",secret"`  // Proofs for accessed bytes
}
```

**Benefits:**
- Constraints: O(m * log n) instead of O(n * m)
- Public input: O(1) instead of O(n)
- Verification gas: Much cheaper on-chain

**Trade-offs:**
- More complex witness preparation
- Need to build Merkle tree off-circuit

## Implementation Plan

### Phase 1: Drop-in Binary Tree (1 day)

1. Add `select_optimized.go`
2. Replace `selectByteAt()` calls
3. Run tests
4. Benchmark constraints

### Phase 2: Adaptive Selection (1 day)

```go
func selectByteAtAdaptive(api frontend.API, tx []frontend.Variable, idx frontend.Variable, maxIdx int) frontend.Variable {
    if maxIdx < 16 {
        // Small arrays: linear is actually faster
        return selectByteAtLinear(api, tx, idx, maxIdx)
    } else if maxIdx < 1000 {
        // Medium arrays: binary tree
        return selectByteAtBinaryTree(api, tx, idx, maxIdx)
    } else {
        // Large arrays: chunked
        return selectByteAtChunked(api, tx, idx, maxIdx, 32)
    }
}
```

### Phase 3: Merkle Tree (1 week)

1. Design new circuit with Merkle commitment
2. Implement witness builder with Merkle tree
3. Update verifyMessage() to use Merkle proofs
4. Comprehensive testing

## Expected Impact

### For typical Cosmos transaction (626 bytes)

**Original circuit:**
- ~150 `selectByteAt()` calls
- Each call: 626 * 3 = 1878 constraints
- **Total: 281,700 constraints from selectByteAt**

**With Binary Tree:**
- ~150 calls
- Each call: log₂(626) * 3 ≈ 30 constraints
- **Total: 4,500 constraints**
- **Reduction: 98.4%!**

**Overall circuit:**
- Original: ~300,000 constraints
- Optimized: ~23,000 constraints
- **13x faster proving time!**

## Conclusion

Binary Tree selection is a **must-have optimization** with:
- ✅ Massive constraints reduction (100x)
- ✅ Easy implementation (drop-in replacement)
- ✅ No changes to witness structure
- ✅ Works for all array sizes

**Recommendation: Implement Binary Tree immediately!**
