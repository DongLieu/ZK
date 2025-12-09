# TxsFieldCircuit - Zero-Knowledge Proof for Cosmos SDK Transactions

## Tổng quan

`TxsFieldCircuit` là một zero-knowledge circuit dùng để chứng minh rằng một transaction bytes từ Cosmos SDK chứa một hoặc nhiều messages với các fields cụ thể, mà không cần tiết lộ toàn bộ nội dung transaction.

## Mục tiêu

**Chứng minh:** "Transaction này chứa N messages, mỗi message có type URL cụ thể và chứa một field với key-value như mong đợi"

**Ứng dụng:**
- Privacy: Chứng minh transaction hợp lệ mà không tiết lộ chi tiết
- Compliance: Verify transaction tuân thủ quy định nhất định
- Selective disclosure: Chỉ công khai một số field cụ thể
- Cross-chain verification: Verify transaction từ chain khác một cách hiệu quả

## Kiến trúc

### 1. Cấu trúc dữ liệu chính

#### `FieldPublic`
```go
type FieldPublic struct {
    Key   frontend.Variable   `gnark:",public"`  // Field number (protobuf)
    Value []frontend.Variable `gnark:",public"`  // Field value bytes
}
```
- **Public input**: Thông tin mà verifier biết
- `Key`: Field number trong protobuf message (ví dụ: field 1, 2, 3...)
- `Value`: Giá trị của field đó (dạng bytes)

#### `MsgAssertion`
```go
type MsgAssertion struct {
    MsgType     []frontend.Variable `gnark:",secret"` // Message type URL
    Field       FieldPublic                            // Field cần verify
    FieldOffset frontend.Variable   `gnark:",secret"` // Vị trí field trong msg
    BodyOffset  frontend.Variable   `gnark:",secret"` // Vị trí msg trong body
}
```
- Mô tả một message cần verify trong transaction
- `MsgType`: TypeURL của message (ví dụ: "/cosmos.bank.v1beta1.MsgSend")
- `FieldOffset`: Vị trí của field trong message value
- `BodyOffset`: Vị trí của message trong TxBody

#### `TxsFieldCircuit`
```go
type TxsFieldCircuit struct {
    TxBytes       []frontend.Variable `gnark:",secret"` // Tx bytes (private)
    PublicTxBytes []frontend.Variable `gnark:",public"` // Tx bytes (public)
    Msgs          []MsgAssertion                        // Messages cần verify

    msgConfigs  []MsgConfig
    txIndexBits int
}
```
- Circuit chính để verify toàn bộ transaction
- Hỗ trợ verify **nhiều messages** trong 1 transaction

### 2. Configuration

#### `MsgConfig`
```go
type MsgConfig struct {
    MsgTypeLen    int  // Độ dài TypeURL (ví dụ: 30 bytes)
    FieldValueLen int  // Độ dài field value (ví dụ: 45 bytes cho address)
    MsgValueLen   int  // Độ dài message value (optional, 0 = không check)
}
```

## Cách hoạt động

### Luồng verification

```
TxBytes (input)
    │
    ├─> Parse TxRaw structure
    │   ├─> Verify tag 0x0a (BodyBytes field)
    │   ├─> Decode varint length
    │   └─> Extract BodyBytes boundaries
    │
    ├─> For each MsgAssertion:
    │   │
    │   ├─> Parse Any wrapper (protobuf)
    │   │   ├─> Verify tag 0x0a (TypeURL)
    │   │   ├─> Verify TypeURL length
    │   │   ├─> Verify TypeURL bytes match MsgType
    │   │   ├─> Verify tag 0x12 (Value)
    │   │   └─> Decode Value length
    │   │
    │   └─> Verify Field in message
    │       ├─> Find field at FieldOffset
    │       ├─> Verify field key matches
    │       ├─> Verify wire type = 2 (length-delimited)
    │       ├─> Decode field length
    │       └─> Verify field value bytes match
    │
    └─> All constraints satisfied ✓
```

## Chi tiết Implementation

### 1. Protobuf Varint Decoding

Circuit hỗ trợ decode protobuf varint (variable-length integer):

```go
func decodeVarintByte(api frontend.API, b frontend.Variable)
    (value frontend.Variable, msb frontend.Variable) {
    bits := api.ToBinary(b, 8)

    // 7 bits thấp là giá trị
    value = sum(bits[0:7] * [1, 2, 4, 8, 16, 32, 64])

    // Bit cao nhất (MSB) = 1 nghĩa là còn byte tiếp theo
    msb = bits[7]
}
```

**Ví dụ:**
- `0x0a` → value=10, msb=0 (single byte)
- `0x8f 0x02` → value=15, msb=1 (byte đầu), value=2, msb=0 (byte thứ 2)
  → Kết quả: 15 + (2 * 128) = 271

### 2. Dynamic Array Indexing

Trong ZK circuits, không thể dùng `array[dynamic_index]` trực tiếp. Circuit dùng kỹ thuật **selector**:

```go
func selectByteAt(api frontend.API, tx []frontend.Variable,
    idx frontend.Variable, maxIdx int) frontend.Variable {

    result := 0
    for pos := 0; pos <= maxIdx; pos++ {
        // Check if pos == idx
        isPos := api.IsZero(api.Sub(idx, pos))

        // If pos == idx, add tx[pos] to result
        result = result + (isPos * tx[pos])
    }
    return result
}
```

**Ví dụ:**
- `idx = 5`, `tx = [10, 20, 30, 40, 50, 60, 70]`
- Loop qua tất cả positions:
  - pos=5: `isPos=1`, result += 1 * 60 = 60
  - pos≠5: `isPos=0`, result += 0 * tx[pos] = 0
- Kết quả: 60

### 3. Protobuf Wire Type Verification

Protobuf field key byte có cấu trúc: `(field_number << 3) | wire_type`

```go
keyBits := api.ToBinary(keyByte, 8)
wireType = keyBits[0] + keyBits[1]*2 + keyBits[2]*4

api.AssertIsEqual(wireType, 2)  // 2 = length-delimited
```

**Wire types:**
- 0: Varint
- 1: 64-bit
- 2: Length-delimited (string, bytes, messages)
- 5: 32-bit

## Ví dụ sử dụng

### 1. Verify MsgSend transaction

```go
// Transaction chứa MsgSend với from_address field
configs := []MsgConfig{
    {
        MsgTypeLen:    30,  // "/cosmos.bank.v1beta1.MsgSend"
        FieldValueLen: 45,  // Cosmos bech32 address length
        MsgValueLen:   0,   // Không check tổng length
    },
}

circuit := NewTxsFieldCircuit(500, configs)

// Assign witness
circuit.TxBytes = txBytes  // Actual transaction
circuit.PublicTxBytes = txBytes
circuit.Msgs[0].MsgType = []byte("/cosmos.bank.v1beta1.MsgSend")
circuit.Msgs[0].Field.Key = 1  // from_address = field 1
circuit.Msgs[0].Field.Value = []byte("cosmos1abc...")
circuit.Msgs[0].FieldOffset = 0  // Field nằm ở offset 0
circuit.Msgs[0].BodyOffset = 2   // Message nằm ở offset 2 trong body
```

### 2. Verify multiple messages

```go
// Transaction chứa 2 messages
configs := []MsgConfig{
    {MsgTypeLen: 30, FieldValueLen: 45},  // Message 1
    {MsgTypeLen: 35, FieldValueLen: 20},  // Message 2
}

circuit := NewTxsFieldCircuit(800, configs)

// Setup message 1
circuit.Msgs[0].MsgType = []byte("/cosmos.bank.v1beta1.MsgSend")
circuit.Msgs[0].Field.Key = 1
circuit.Msgs[0].Field.Value = []byte("cosmos1sender...")
circuit.Msgs[0].BodyOffset = 2

// Setup message 2
circuit.Msgs[1].MsgType = []byte("/cosmos.staking.v1beta1.MsgDelegate")
circuit.Msgs[1].Field.Key = 2
circuit.Msgs[1].Field.Value = []byte("validator_address")
circuit.Msgs[1].BodyOffset = 150  // Message thứ 2 ở vị trí sau
```

## Cosmos SDK Transaction Structure

### TxRaw (protobuf)
```protobuf
message TxRaw {
  bytes body_bytes = 1;        // tag: 0x0a
  bytes auth_info_bytes = 2;   // tag: 0x12
  repeated bytes signatures = 3; // tag: 0x1a
}
```

### TxBody
```protobuf
message TxBody {
  repeated google.protobuf.Any messages = 1;  // tag: 0x0a
  string memo = 2;
  uint64 timeout_height = 3;
  // ...
}
```

### Any (message wrapper)
```protobuf
message Any {
  string type_url = 1;  // tag: 0x0a, ví dụ: "/cosmos.bank.v1beta1.MsgSend"
  bytes value = 2;      // tag: 0x12, protobuf encoded message
}
```

### MsgSend
```protobuf
message MsgSend {
  string from_address = 1;  // tag: 0x0a
  string to_address = 2;    // tag: 0x12
  repeated Coin amount = 3; // tag: 0x1a
}
```

## Constraints Analysis

Circuit tạo ra các constraints để verify:

### 1. Structure constraints
- TxBytes[0] = 0x0a (BodyBytes tag)
- Varint decoding đúng format
- Boundaries hợp lệ (offsets không vượt quá length)

### 2. Message constraints (per message)
- Message tag = 0x0a
- TypeURL length và content khớp MsgType
- Value tag = 0x12
- Field key khớp
- Field wire type = 2
- Field value khớp với public input

### 3. Ordering constraints
- Messages có thứ tự tăng dần (BodyOffset[i] <= BodyOffset[i+1])
- Offsets nằm trong boundaries hợp lệ

### Ước tính số constraints

Với 1 message trong transaction 500 bytes:
- Base constraints: ~100
- Per-message constraints: ~1000-2000 (tùy độ dài fields)
- Dynamic indexing overhead: ~500 * số lần gọi selectByteAt

**Tổng:** ~3000-5000 constraints cho 1 message

## Ưu điểm

✅ **Flexible**: Hỗ trợ nhiều messages với configs khác nhau
✅ **Complete**: Parse đầy đủ protobuf structure (varint, wire types)
✅ **Safe**: Range checks và boundary validations đầy đủ
✅ **Efficient**: Dynamic indexing được optimize
✅ **Extensible**: Dễ dàng thêm message types và fields mới

## Hạn chế và cải tiến

### Hạn chế hiện tại

1. **Fixed-size arrays**: TxBytes và Field.Value phải có size cố định lúc compile
2. **Linear complexity**: selectByteAt() có O(n) constraints cho mỗi lookup
3. **No nested messages**: Chỉ verify 1 level của message structure
4. **Limited varint**: Chỉ hỗ trợ varint 2 bytes (max value ~16K)

### Cải tiến có thể làm

1. **Merkle tree indexing**: Thay selectByteAt bằng Merkle proof để giảm constraints
2. **Recursive structures**: Hỗ trợ verify nested messages
3. **Batch verification**: Verify nhiều transactions trong 1 proof
4. **Signature verification**: Thêm ECDSA/EdDSA signature verification
5. **Hash commitments**: Public input là hash của TxBytes thay vì toàn bộ bytes

## Testing

### Unit tests

```bash
go test -v ./txscircuit
```

### Integration test với real transaction

```go
// Tạo real Cosmos SDK transaction
tx := createCosmosTransaction()
txBytes, _ := proto.Marshal(tx)

// Extract thông tin cần verify
fromAddr := extractFromAddress(tx)

// Setup circuit
circuit := NewTxsFieldCircuit(len(txBytes), configs)
// ... assign witness ...

// Compile và generate proof
ccs, _ := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
pk, vk, _ := groth16.Setup(ccs)
proof, _ := groth16.Prove(ccs, pk, witness)
err := groth16.Verify(proof, vk, publicWitness)
```

## References

- [Cosmos SDK Transactions](https://docs.cosmos.network/main/core/transactions)
- [Protocol Buffers Encoding](https://protobuf.dev/programming-guides/encoding/)
- [gnark ZK Framework](https://docs.gnark.consensys.net/)
- [Groth16 Proof System](https://eprint.iacr.org/2016/260.pdf)

## License

MIT License

## Contributing

Contributions are welcome! Vui lòng tạo issue hoặc pull request.
