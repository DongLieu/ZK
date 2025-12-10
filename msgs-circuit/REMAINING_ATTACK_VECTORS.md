# Các Kịch Bản Tấn Công Còn Lại

## Tóm Tắt

Sau khi fix **Attack #4 (Message Skipping)**, vẫn còn **6 lỗ hổng nghiêm trọng** chưa được khắc phục. Đặc biệt, **5/7 attack vectors vẫn hoạt động ngay cả khi có signature verification**.

| Attack | Severity | Blocked by Signature? | Status |
|--------|----------|----------------------|--------|
| #1 Field Overlap/Aliasing | 🔴 CRITICAL | ❌ NO | ⚠️ UNFIXED |
| #2 Field Boundary Bypass | 🔴 CRITICAL | ❌ NO | ⚠️ UNFIXED |
| #3 Varint Non-canonical | 🟠 HIGH | ❌ NO | ⚠️ UNFIXED |
| #4 Message Skipping | 🔴 CRITICAL | ✅ YES | ✅ **FIXED** |
| #5 Memo Poisoning | 🟠 HIGH | ⚠️ PARTIAL | ⚠️ UNFIXED |
| #6 Dynamic Size Bypass | 🟡 MEDIUM | ❌ NO | ⚠️ UNFIXED |
| #7 Field Number Mismatch | 🔴 CRITICAL | ❌ NO | ⚠️ UNFIXED |

---

## 🔴 ATTACK #1: Field Overlap/Aliasing

### Mô Tả
Circuit không kiểm tra xem các field được verify có overlap (chồng chéo) vị trí với nhau hay không. Kẻ tấn công có thể trỏ nhiều field khác nhau vào **cùng một vùng dữ liệu** trong message.

### Vị Trí Lỗ Hổng
**File:** `msgs-circuit/txscircuit/circuit.go`
**Lines:** 142-174

```go
// Circuit chỉ check:
api.AssertIsLessOrEqual(msg.FieldOffset, valueLen)  // Line 142

// Nhưng KHÔNG check field này có overlap với field khác không!
```

### Kịch Bản Tấn Công Chi Tiết

#### Step 1: Signer tạo transaction hợp lệ
```protobuf
message MsgSend {
  string from_address = 1;  // Offset: 0-44  (45 bytes)
  string to_address = 2;    // Offset: 45-89 (45 bytes)
  repeated Coin amount = 3; // Offset: 90-120 (31 bytes)
}

// Serialized structure:
// [0x0A][len][from_address_data...]  ← Field 1
// [0x12][len][to_address_data...]    ← Field 2
// [0x1A][len][amount_data...]        ← Field 3
```

#### Step 2: Transaction được ký hợp lệ
```go
// Tx có signature hợp lệ
txBytes = sign(TxBody{
    Messages: [MsgSend{
        from: "cosmos1abc...",
        to:   "cosmos1xyz...",
        amount: [Coin{denom: "uatom", amount: "1000"}]
    }]
})
```

#### Step 3: Attacker tạo circuit với 2 field assertions
```go
configs := []MsgConfig{{
    MsgTypeLen: 40,
    FieldValueLen: 20,  // Field 1
}, {
    MsgTypeLen: 40,
    FieldValueLen: 30,  // Field 2
}}

// Attacker claim trong witness:
Msgs[0].Field.Key = 0x1A  // Field 3 tag (amount)
Msgs[0].FieldOffset = 90  // Correct offset for field 3
Msgs[0].Field.Value = [real_amount_data]

Msgs[1].Field.Key = 0x12  // Field 2 tag (to_address)
Msgs[1].FieldOffset = 90  // ← SAME OFFSET! Overlap!
Msgs[1].Field.Value = [fake_address_data]
```

#### Step 4: Circuit verification
```go
// Check Field 1 (amount):
// - Offset 90 + len(amount) = 90 + 31 = 121 <= valueLen ✓
// - Value matches at offset 90 ✓

// Check Field 2 (to_address):
// - Offset 90 + len(to_address) = 90 + 30 = 120 <= valueLen ✓
// - Value matches at offset 90 ✓

// → BOTH PASS! But they overlap at offset 90!
```

### Hậu Quả
- Smart contract nhận được 2 proofs về **cùng một vùng dữ liệu**
- Proof 1 claim: "vùng này là amount = 1000 ATOM"
- Proof 2 claim: "vùng này là to_address = cosmos1fake..."
- **Logic mâu thuẫn nhưng circuit accept!**

### Proof of Concept
```go
// Test case demonstrating field overlap
func TestFieldOverlapAttack(t *testing.T) {
    // Create tx with MsgSend
    msgBytes := buildMsgSend("cosmos1from", "cosmos1to", "1000uatom")

    // Create circuit with 2 field assertions
    circuit := NewTxsFieldCircuit(len(txBytes), []MsgConfig{
        {FieldValueLen: 20}, // Field 1
        {FieldValueLen: 30}, // Field 2
    })

    // Malicious witness: both fields point to offset 90
    witness.Msgs[0].FieldOffset = 90  // amount field
    witness.Msgs[1].FieldOffset = 90  // OVERLAP!

    // This should fail but currently passes
    assert.ProverSucceeded(circuit, witness) // ← VULNERABLE!
}
```

### Khắc Phục
```go
// Add overlap detection after verifying all fields
for i := 0; i < len(circuit.Msgs); i++ {
    for j := i + 1; j < len(circuit.Msgs); j++ {
        // Calculate field 1 range: [offset1, offset1 + size1)
        field1Start := circuit.Msgs[i].FieldOffset
        field1End := api.Add(field1Start, totalField1Size)

        // Calculate field 2 range: [offset2, offset2 + size2)
        field2Start := circuit.Msgs[j].FieldOffset
        field2End := api.Add(field2Start, totalField2Size)

        // Assert no overlap:
        // (field1End <= field2Start) OR (field2End <= field1Start)
        noOverlap := api.Or(
            api.IsLessOrEqual(field1End, field2Start),
            api.IsLessOrEqual(field2End, field1Start),
        )
        api.AssertIsEqual(noOverlap, 1)
    }
}
```

---

## 🔴 ATTACK #2: Field Boundary Bypass

### Mô Tả
Circuit kiểm tra field không vượt quá `valueLen` nhưng **KHÔNG** kiểm tra field có nằm đúng vị trí so với các field khác trong protobuf structure.

### Vị Trí Lỗ Hổng
**File:** `msgs-circuit/txscircuit/circuit.go`
**Line:** 174

```go
api.AssertIsLessOrEqual(api.Add(msg.FieldOffset, totalField), valueLen)

// Chỉ check: offset + size <= valueLen
// KHÔNG check: offset có phải vị trí đúng của field này không
```

### Kịch Bản Tấn Công Chi Tiết

#### Protobuf Structure
```protobuf
message MsgDelegate {
  string delegator_address = 1;  // Expected offset: 0-44
  string validator_address = 2;  // Expected offset: 45-89
  Coin amount = 3;               // Expected offset: 90-120
}

// Real serialized data:
// Bytes [0-44]:   delegator_address = "cosmos1delegator123..."
// Bytes [45-89]:  validator_address = "cosmosvaloper1val..."
// Bytes [90-120]: amount = {denom: "stake", amount: "1000"}
```

#### Attack Step 1: Create legitimate transaction
```go
tx := MsgDelegate{
    DelegatorAddress: "cosmos1delegator123...",
    ValidatorAddress: "cosmosvaloper1val...",
    Amount:           Coin{Denom: "stake", Amount: "1000"},
}
txBytes = sign(tx) // Valid signature
```

#### Attack Step 2: Malicious circuit witness
```go
// Attacker claims to prove field 3 (amount)
witness.Field.Key = 0x1A  // Field 3 tag: (3 << 3) | 2

// But provides WRONG offset - pointing to delegator_address!
witness.FieldOffset = 10  // ← Should be 90, but points to 10!

// Provide fake amount data that matches bytes 10-40
witness.Field.Value = extractBytes(txBytes, 10, 30)

// Circuit checks:
// 10 + 30 = 40 <= 120 (valueLen) ✓ PASS!
//
// But actual amount field is at offset 90, not 10!
```

#### Step 3: Exploitation
```go
// Smart contract receives proof:
// "Field 3 (amount) = <data_from_offset_10>"

// But offset 10 is actually part of delegator_address!
// Attacker proved WRONG field but circuit accepted it.
```

### Visualizing the Attack
```
Real Message Structure:
┌──────────────────┬──────────────────┬──────────────────┐
│ Field 1          │ Field 2          │ Field 3          │
│ delegator        │ validator        │ amount           │
│ Offset: 0-44     │ Offset: 45-89    │ Offset: 90-120   │
└──────────────────┴──────────────────┴──────────────────┘

Attacker's Claim:
┌──────────────────┬──────────────────┬──────────────────┐
│ Field 1          │ Field 2          │ Field 3          │
│ delegator        │ validator        │ amount           │
│                  │                  │                  │
│        ▲─────────┼──────Field 3?────┤                  │
│    Offset: 10    │   (WRONG!)       │   (Not checked)  │
└──────────────────┴──────────────────┴──────────────────┘
```

### Hậu Quả
- Proof claim về field 3 (amount) nhưng thực tế đọc data từ field 1 (delegator)
- Data mismatch giữa field number và actual data
- Có thể bypass amount validation bằng cách trỏ vào field khác

### Khắc Phục
```go
// Need to verify field appears at correct position relative to other fields
// Option 1: Strict protobuf parsing - decode all fields sequentially
// Option 2: Field ordering constraint

// Require fields to be verified in protobuf field number order
for i := 1; i < len(circuit.Msgs); i++ {
    prevFieldNum := extractFieldNumber(circuit.Msgs[i-1].Field.Key)
    currFieldNum := extractFieldNumber(circuit.Msgs[i].Field.Key)

    // Current field number must be greater
    api.AssertIsLessOrEqual(prevFieldNum, currFieldNum)

    // Current offset must be after previous field end
    prevFieldEnd := calculateFieldEnd(circuit.Msgs[i-1])
    api.AssertIsLessOrEqual(prevFieldEnd, circuit.Msgs[i].FieldOffset)
}
```

---

## 🟠 ATTACK #3: Varint Non-canonical Encoding

### Mô Tả
Protobuf varint cho phép encode cùng một số bằng nhiều cách khác nhau (canonical và non-canonical). Circuit decode đúng giá trị nhưng không enforce canonical encoding, dẫn đến attacker có thể manipulate offset của các field sau đó.

### Vị Trí Lỗ Hổng
**File:** `msgs-circuit/txscircuit/circuit.go`
**Lines:** 196-244 (decodeVarint4Bytes)

```go
func decodeVarint4Bytes(api frontend.API, tx []frontend.Variable, startIdx frontend.Variable, maxIdx int) (frontend.Variable, frontend.Variable) {
    // Decodes varint correctly but accepts ANY valid encoding
    // Does NOT enforce shortest encoding
}
```

### Varint Encoding Basics
```
Value 127 có thể encode thành:

Canonical (shortest):
  [0x7F]                    // 1 byte: 01111111

Non-canonical (longer):
  [0xFF, 0x00]              // 2 bytes: 11111111 00000000
  [0xFF, 0x80, 0x00]        // 3 bytes: 11111111 10000000 00000000
  [0xFF, 0x80, 0x80, 0x00]  // 4 bytes: ...

Tất cả đều decode thành 127!
```

### Kịch Bản Tấn Công Chi Tiết

#### Step 1: Attacker crafts message với non-canonical varint
```protobuf
message MsgSend {
  string from_address = 1;
  string to_address = 2;
  repeated Coin amount = 3;
}

// Normal encoding (canonical):
[0x0A][0x2C][...from_address 44 bytes...]
[0x12][0x2C][...to_address 44 bytes...]
[0x1A][0x1F][...amount 31 bytes...]

// Malicious encoding (non-canonical):
[0x0A][0xFF, 0x80, 0x80, 0x00][...from_address 44 bytes...]  ← 4 bytes for len=44!
[0x12][0x2C][...to_address 44 bytes...]
[0x1A][0x1F][...amount 31 bytes...]

// Both are VALID protobuf!
// Both have signature validation!
```

#### Step 2: Offset manipulation
```go
// Canonical encoding:
// - Field 1 tag at offset 0
// - Field 1 len at offset 1 (1 byte)
// - Field 1 data at offset 2-45 (44 bytes)
// - Field 2 tag at offset 46

// Non-canonical encoding:
// - Field 1 tag at offset 0
// - Field 1 len at offset 1 (4 bytes!)
// - Field 1 data at offset 5-48 (44 bytes)
// - Field 2 tag at offset 49  ← 3 bytes later!

// All subsequent field offsets are shifted by 3 bytes!
```

#### Step 3: Proof manipulation
```go
// Attacker can claim two different proofs for same tx:

// Proof 1: Using canonical interpretation
witness1.Msgs[0].FieldOffset = 46  // Field 2 at offset 46

// Proof 2: Using non-canonical interpretation
witness2.Msgs[0].FieldOffset = 49  // Field 2 at offset 49

// Both pass circuit verification!
// But they claim different field locations for same tx!
```

### Advanced Attack: Offset Confusion
```go
// Scenario: Tx with 3 messages
// Message 1: Normal encoding
// Message 2: Non-canonical varint for length
// Message 3: Normal encoding

// Circuit expects Message 3 at offset X
// But due to non-canonical varint in Message 2,
// Message 3 is actually at offset X+N

// Attacker can:
// 1. Provide incorrect offset that passes validation
// 2. Point to different data than expected
// 3. Create multiple valid proofs for same tx with different interpretations
```

### Hậu Quả
- Cùng một transaction có thể tạo ra nhiều proof khác nhau
- Offset của các field bị lệch so với expected
- Có thể bypass field verification bằng cách shift offsets
- Non-deterministic proof generation

### Khắc Phục
```go
// Enforce canonical varint encoding
func decodeVarintCanonical(api frontend.API, tx []frontend.Variable, startIdx frontend.Variable, maxIdx int) (frontend.Variable, frontend.Variable) {
    byte1 := selectByteAt(api, tx, startIdx, maxIdx)
    val1, msb1 := decodeVarintByte(api, byte1)

    // If MSB is 0 (single byte varint), ensure value < 128
    isSingleByte := api.IsZero(msb1)

    // If single byte, value must be <= 127
    // This is automatically satisfied

    // If MSB is 1 (multi-byte varint), ensure value >= 128
    byte2 := selectByteAt(api, tx, api.Add(startIdx, 1), maxIdx)
    val2, msb2 := decodeVarintByte(api, byte2)

    // If using 2 bytes, value must be >= 128
    // value = val1 + 128*val2
    // Require: val1 + 128*val2 >= 128
    // Which means: val2 >= 1 OR val1 >= 128 (but val1 < 128 always)
    // So: val2 >= 1

    isMultiByte := api.Sub(1, isSingleByte)
    mustBeAtLeast1 := api.Mul(isMultiByte, val2)
    api.AssertIsLessOrEqual(isMultiByte, mustBeAtLeast1)

    // Similar checks for 3-byte and 4-byte varints
    // ...
}
```

---

## 🟠 ATTACK #5: Memo/Extension Poisoning

### Mô Tả
Circuit chỉ verify `messages` field trong TxBody nhưng không verify `memo`, `timeout_height`, hay `extension_options`. Attacker (hoặc chính signer) có thể inject malicious data vào các field này.

### TxBody Structure
```protobuf
message TxBody {
  repeated google.protobuf.Any messages = 1;        // ← Circuit verifies
  string memo = 2;                                   // ← NOT verified!
  uint64 timeout_height = 3;                        // ← NOT verified!
  repeated google.protobuf.Any extension_options = 1023; // ← NOT verified!
}
```

### Kịch Bản Tấn Công Chi Tiết

#### Attack Type 5A: Malicious Memo
```go
// Signer tạo tx với malicious memo
txBody := &txtypes.TxBody{
    Messages: []*codectypes.Any{
        legitimateMsgSend, // Amount: 100 ATOM (small, legitimate)
    },

    // Inject malicious smart contract call data in memo
    Memo: encodeSmartContractCall(
        contractAddr: "cosmos1malicious...",
        method: "drain_funds",
        params: {amount: "999999999"},
    ),
}

// Transaction gets signed (signature valid!)
// Circuit proves legitimateMsgSend (100 ATOM)
// But memo contains instructions to drain 999999999 tokens!
```

#### Attack Type 5B: Large Memo DoS
```go
// Attacker creates tx with huge memo
txBody := &txtypes.TxBody{
    Messages: []*codectypes.Any{msgSend},
    Memo: strings.Repeat("A", 10*1024*1024), // 10MB memo!
}

// Circuit verifies messages correctly
// But doesn't check memo size
// Can cause:
// - Storage bloat
// - Proof generation OOM
// - Network congestion
```

#### Attack Type 5C: Extension Options Exploit
```go
// Attacker uses extension_options to inject malicious behavior
txBody := &txtypes.TxBody{
    Messages: []*codectypes.Any{msgDelegate},

    ExtensionOptions: []*codectypes.Any{
        {
            TypeUrl: "/custom.ExtensionMalicious",
            Value: encodeMaliciousExtension(
                // Hidden message that executes after delegation
                hiddenMsg: MsgSend{to: attacker, amount: "99999"},
            ),
        },
    },
}
```

### Status: Partially Fixed
Fix cho Attack #4 đã ngăn chặn **external** memo modification (vì signature), nhưng **KHÔNG** ngăn chặn:
- Signer cố tình đưa malicious memo
- Application logic dựa vào memo data mà không validate
- Smart contract đọc memo từ proof

### Hậu Quả
- Malicious data trong memo không được circuit kiểm tra
- Smart contract có thể đọc memo và thực thi malicious logic
- Storage/DoS attacks với memo lớn
- Hidden messages trong extension_options

### Khắc Phục

#### Option 1: Verify Memo in Circuit (Strict)
```go
// Add memo verification to circuit
type TxsFieldCircuit struct {
    // ... existing fields
    MemoMaxLen int
    AllowedMemoPattern []byte  // Regex or whitelist
}

func (circuit *TxsFieldCircuit) Define(api frontend.API) error {
    // ... existing verification

    // After verifying all messages, parse memo field
    if cursor < bodyEnd {
        nextTag := selectByteAt(api, tx, cursor, maxIdx)

        // If next field is memo (tag 0x12)
        isMemoTag := api.IsZero(api.Sub(nextTag, 0x12))

        // Parse memo length
        memoLenIdx := api.Add(cursor, 1)
        memoLen := decodeVarint4Bytes(api, tx, memoLenIdx, maxIdx)

        // Enforce max memo length
        api.AssertIsLessOrEqual(memoLen, frontend.Variable(circuit.MemoMaxLen))

        // Optionally: verify memo contains only allowed characters
        // ...
    }
}
```

#### Option 2: Application-Level Validation (Flexible)
```go
// Smart contract validates memo separately
func validateProof(proof ZKProof, tx Transaction) bool {
    // 1. Verify ZK proof (messages only)
    if !verifyZKProof(proof) {
        return false
    }

    // 2. Separately validate memo
    if len(tx.Memo) > MAX_MEMO_SIZE {
        return false
    }

    if containsMaliciousPatterns(tx.Memo) {
        return false
    }

    // 3. Verify no extension options
    if len(tx.ExtensionOptions) > 0 {
        return false
    }

    return true
}
```

#### Option 3: Document Limitation
```
# Circuit Scope
This circuit ONLY verifies:
- TxBody.messages field
- Specific fields within each message

This circuit DOES NOT verify:
- TxBody.memo
- TxBody.timeout_height
- TxBody.extension_options

Applications MUST validate these fields separately!
```

---

## 🟡 ATTACK #6: Dynamic Size Bypass

### Mô Tả
Khi `MsgValueLen = 0` (dynamic size mode), circuit không verify độ dài thực tế của message value, cho phép attacker truncate hoặc claim sai kích thước message.

### Vị Trí Lỗ Hổng
**File:** `msgs-circuit/txscircuit/circuit.go`
**Lines:** 135-137

```go
if cfg.MsgValueLen > 0 {
    api.AssertIsEqual(valueLen, frontend.Variable(cfg.MsgValueLen))
}
// Nếu MsgValueLen = 0, không có check nào cả!
```

### Kịch Bản Tấn Công Chi Tiết

#### Setup: Circuit with dynamic size
```go
configs := []MsgConfig{{
    MsgTypeLen: 40,
    FieldValueLen: 20,
    MsgValueLen: 0,  // ← Dynamic size, no length check!
}}

circuit := NewTxsFieldCircuit(txLen, configs)
```

#### Attack Step 1: Create legitimate transaction
```protobuf
message MsgSend {
  string from_address = 1;  // 44 bytes
  string to_address = 2;    // 44 bytes
  repeated Coin amount = 3; // 31 bytes
  string memo = 4;          // 50 bytes
  uint64 flags = 5;         // 10 bytes
}

// Total message value size: 179 bytes
```

#### Attack Step 2: Claim truncated size
```go
// Attacker claims in witness:
witness.Msgs[0].ValueLen = 150  // ← Truncated! Real size is 179

// Circuit doesn't verify because MsgValueLen = 0
// Attacker proves only fields 1-3, ignoring fields 4-5

// Fields 4-5 (last 29 bytes) are hidden from verification!
```

#### Attack Step 3: Selective field proof
```go
// Real message has 5 fields:
// Field 1: from_address (offset 0)
// Field 2: to_address (offset 45)
// Field 3: amount (offset 90)
// Field 4: memo (offset 121) ← Hidden!
// Field 5: flags (offset 171) ← Hidden!

// Attacker only proves field 3 (amount)
// Circuit accepts because:
// - Field 3 is within truncated valueLen (150)
// - No check on actual message size
// - Fields 4-5 are never verified
```

### Advanced Attack: Overflow Manipulation
```go
// If MsgValueLen = 0 and no bounds checking:
witness.Msgs[0].ValueLen = 9999999  // Claim huge size

// If circuit doesn't validate against actual data:
// - Can claim fields exist at invalid offsets
// - Can bypass boundary checks
// - Can overlap with other messages
```

### Hậu Quả
- Attacker bỏ qua verification của các field cuối trong message
- Có thể hide malicious fields
- Size manipulation dẫn đến offset confusion
- Inconsistent interpretation giữa prover và verifier

### Khắc Phục
```go
// Option 1: Always require MsgValueLen
func NewTxsFieldCircuit(txLen int, configs []MsgConfig) *TxsFieldCircuit {
    for _, cfg := range configs {
        if cfg.MsgValueLen == 0 {
            panic("MsgValueLen must be specified (dynamic size not allowed)")
        }
    }
    // ...
}

// Option 2: Verify actual size even in dynamic mode
if cfg.MsgValueLen > 0 {
    api.AssertIsEqual(valueLen, frontend.Variable(cfg.MsgValueLen))
} else {
    // Dynamic mode: at least verify size is reasonable
    api.AssertIsLessOrEqual(valueLen, frontend.Variable(MAX_MSG_SIZE))

    // And verify no more fields after verified field
    lastFieldEnd := api.Add(
        api.Add(msg.FieldOffset, frontend.Variable(1)),
        api.Add(fieldBytes, frontend.Variable(len(msg.Field.Value))),
    )

    // All remaining bytes must be continuation of known fields
    // or padding, not new fields
    // ... additional checks
}
```

---

## 🔴 ATTACK #7: Field Number Mismatch

### Mô Tả
Circuit chỉ verify wire type = 2 (length-delimited) và key < 128, nhưng **KHÔNG** verify field number có match với expected field. Attacker có thể prove field sai nhưng circuit vẫn accept.

### Vị Trí Lỗ Hổng
**File:** `msgs-circuit/txscircuit/circuit.go`
**Lines:** 149-156

```go
keyBits := api.ToBinary(keyByte, 8)
api.AssertIsEqual(keyBits[7], 0)  // MSB must be 0

wireType := api.Add(
    keyBits[0],
    api.Mul(keyBits[1], frontend.Variable(2)),
    api.Mul(keyBits[2], frontend.Variable(4)),
)
api.AssertIsEqual(wireType, 2)  // Wire type = length-delimited

// Nhưng KHÔNG check field number!
// Tag format: (field_number << 3) | wire_type
// Chỉ verify wire_type, không verify field_number
```

### Protobuf Tag Format
```
Tag byte = (field_number << 3) | wire_type

Examples:
- Field 1, wire type 2: 0x0A = (1 << 3) | 2 = 8 + 2 = 10
- Field 2, wire type 2: 0x12 = (2 << 3) | 2 = 16 + 2 = 18
- Field 3, wire type 2: 0x1A = (3 << 3) | 2 = 24 + 2 = 26
- Field 5, wire type 2: 0x2A = (5 << 3) | 2 = 40 + 2 = 42

All have wire type 2, but different field numbers!
```

### Kịch Bản Tấn Công Chi Tiết

#### Step 1: Expected proof configuration
```go
// Application expects proof of field 3 (amount)
expectedFieldNum := 3
expectedTag := byte((3 << 3) | 2) // 0x1A

// Circuit configured to verify amount field
circuit := NewTxsFieldCircuit(txLen, []MsgConfig{{
    MsgTypeLen: 40,
    FieldValueLen: 20,  // Size of amount field
}})
```

#### Step 2: Real transaction structure
```protobuf
message MsgSend {
  string from_address = 1;  // Tag 0x0A, wire type 2
  string to_address = 2;    // Tag 0x12, wire type 2
  repeated Coin amount = 3; // Tag 0x1A, wire type 2
  string memo = 5;          // Tag 0x2A, wire type 2
}

// Serialized:
[0x0A][len][from_address_data...]
[0x12][len][to_address_data...]
[0x1A][len][amount_data...]        ← Expected to verify THIS
[0x2A][len][memo_data...]
```

#### Step 3: Attacker provides wrong field
```go
// Attacker submits witness claiming to verify field 3 (amount)
// But actually provides field 5 (memo) data!

witness.Field.Key = 0x2A  // Field 5 tag! Not field 3!
witness.FieldOffset = <offset_of_memo_field>
witness.Field.Value = memoData

// Circuit verification:
keyByte = 0x2A = 0b00101010

// Check MSB = 0:
keyBits[7] = 0 ✓ PASS

// Check wire type = 2:
wireType = keyBits[0] + 2*keyBits[1] + 4*keyBits[2]
        = 0 + 2*1 + 4*0 = 2 ✓ PASS

// Field number check:
// NOT PERFORMED! ← VULNERABILITY

// Circuit accepts field 5 as if it's field 3!
```

#### Step 4: Exploitation
```go
// Application expects proof of amount field (field 3)
// But received proof of memo field (field 5)

// Smart contract logic:
if verifyProof(proof, expectedFieldTag: 0x1A) {
    // Expects: proof verified amount = X
    // Reality: proof verified memo = Y

    // Process based on wrong field data!
    processAmount(proof.fieldValue)  // Actually memo data!
}
```

### Visualizing Field Number Extraction
```
Tag byte: 0x2A = 0b00101010

Bits: [0][1][0][1][0][1][0][0]
       ^  ^  ^  ^  ^  ^  ^  ^
       7  6  5  4  3  2  1  0

Wire type (bits 0-2): [0][1][0] = 2 ✓
Field number (bits 3-7): [0][1][0][1][0] = 5

Expected field number: 3 = 0b00011
Actual field number:   5 = 0b00101
                           ^^^^^^^^ MISMATCH!
```

### Advanced Attack: Field Confusion
```go
// Scenario: Circuit configured to verify 2 fields
// Expected:
// - Field 1: from_address (tag 0x0A)
// - Field 3: amount (tag 0x1A)

// Attacker provides:
// - Field 2: to_address (tag 0x12)
// - Field 5: memo (tag 0x2A)

// Both have wire type 2, so circuit accepts them
// But they're DIFFERENT fields than expected!

configs := []MsgConfig{
    {FieldValueLen: 44},  // Expect field 1
    {FieldValueLen: 20},  // Expect field 3
}

witness.Msgs[0].Field.Key = 0x12  // Actually field 2!
witness.Msgs[1].Field.Key = 0x2A  // Actually field 5!

// Circuit: ✓ Both have wire type 2
// Reality: Wrong fields verified!
```

### Hậu Quả
- Circuit verify sai field nhưng claim đúng field
- Application logic dựa vào wrong field data
- Có thể bypass amount checks bằng cách prove memo field
- Type confusion attacks

### Khắc Phục
```go
// Solution 1: Add expected field number to public inputs
type FieldPublic struct {
    Key          frontend.Variable `gnark:",public"`
    FieldNumber  frontend.Variable `gnark:",public"`  // ← Add this
    Value        []frontend.Variable `gnark:",public"`
}

func (circuit *TxsFieldCircuit) verifyMessage(...) {
    keyByte := selectByteAt(api, tx, fieldStart, maxIdx)
    api.AssertIsEqual(keyByte, msg.Field.Key)

    // Extract field number from key
    keyBits := api.ToBinary(keyByte, 8)
    fieldNumber := api.Add(
        api.Mul(keyBits[3], 1),
        api.Mul(keyBits[4], 2),
        api.Mul(keyBits[5], 4),
        api.Mul(keyBits[6], 8),
        api.Mul(keyBits[7], 16),
    )

    // Verify field number matches expected
    api.AssertIsEqual(fieldNumber, msg.Field.FieldNumber)
}

// Solution 2: Use tag lookup table
var expectedFieldTags = map[string]byte{
    "from_address": 0x0A,  // Field 1
    "to_address":   0x12,  // Field 2
    "amount":       0x1A,  // Field 3
}

// In circuit config, specify exact expected tag
type MsgConfig struct {
    MsgTypeLen    int
    FieldValueLen int
    ExpectedTag   byte  // ← Enforce specific tag
}

// In verification:
api.AssertIsEqual(keyByte, frontend.Variable(cfg.ExpectedTag))
```

---

## 📊 Attack Comparison Matrix

| Attack | Can Modify TX? | Needs Signature? | Exploits Circuit? | Exploits Protobuf? |
|--------|---------------|------------------|-------------------|-------------------|
| #1 Field Overlap | ❌ No | ✅ Yes | ✅ Yes | ❌ No |
| #2 Boundary Bypass | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes |
| #3 Non-canonical Varint | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |
| #4 Message Skipping | ❌ No | ✅ Yes | ✅ Yes | ❌ No |
| #5 Memo Poison | ✅ Yes | ✅ Yes | ⚠️ Partial | ❌ No |
| #6 Dynamic Size | ❌ No | ✅ Yes | ✅ Yes | ❌ No |
| #7 Field Number | ❌ No | ✅ Yes | ✅ Yes | ⚠️ Partial |

## 🎯 Recommended Fix Priority

### Phase 1: Critical Fixes (Immediate)
1. **Attack #7**: Field Number Mismatch - Easiest to fix, high impact
2. **Attack #1**: Field Overlap - Prevent logical contradictions
3. **Attack #2**: Boundary Bypass - Prevent wrong field access

### Phase 2: Important Fixes (Short-term)
4. **Attack #3**: Non-canonical Varint - Prevent offset manipulation
5. **Attack #6**: Dynamic Size Bypass - Enforce size constraints

### Phase 3: Application-Level Fixes (Medium-term)
6. **Attack #5**: Memo Poisoning - Document limitations, add app-level validation

## 🔒 Defense in Depth Strategy

```
Layer 1: Signature Verification
├─ Prevents: External TX modification
└─ Blocks: Attack #4 (Message Skipping)

Layer 2: Circuit Constraints ← CURRENT FOCUS
├─ Prevents: Invalid circuit proofs
├─ Currently blocks: Attack #4
└─ Needs fixes for: Attacks #1, #2, #3, #6, #7

Layer 3: Semantic Validation ← RECOMMENDED
├─ Prevents: Logical inconsistencies
├─ Should validate: Field overlap, field order, canonical encoding
└─ Application-specific: Memo size, allowed patterns

Layer 4: Application Logic ← APPLICATION RESPONSIBILITY
├─ Validates: Business logic constraints
├─ Checks: Amount limits, address whitelist, etc.
└─ Handles: Attack #5 (Memo poisoning)
```

## 📝 Testing Recommendations

Mỗi attack cần ít nhất 3 test cases:
1. **Positive test**: Legitimate proof should pass
2. **Negative test**: Attack should fail
3. **Edge case test**: Boundary conditions

## 🚨 Security Notice

**CRITICAL**: Hệ thống hiện tại chỉ an toàn với Attack #4. **KHÔNG** sử dụng trong production cho đến khi fix hết các attack còn lại!

Ngay cả với signature verification, **5/7 attacks vẫn hoạt động**. Circuit cần thêm constraints để đảm bảo semantic correctness, không chỉ structural validity.
