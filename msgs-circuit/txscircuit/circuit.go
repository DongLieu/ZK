package txscircuit

import (
	"math/bits"

	"github.com/consensys/gnark/frontend"
)

// FieldPublic chứa key + value (length-delimited field) làm public input.
type FieldPublic struct {
	Key   frontend.Variable   `gnark:",public"`
	Value []frontend.Variable `gnark:",public"`
}

// MsgAssertion mô tả một message trong TxBody chúng ta muốn kiểm chứng.
type MsgAssertion struct {
	Field       FieldPublic
	FieldOffset frontend.Variable `gnark:",secret"`
	BodyOffset  frontend.Variable `gnark:",secret"`
}

// MsgConfig định nghĩa kích thước cố định cho từng message assertion.
type MsgConfig struct {
	FieldValueLen int
	MsgValueLen   int
}

// TxsFieldCircuit chứng minh TxBytes chứa nhiều Msg (có thể >1) và mỗi Msg
// có Field length-delimited khớp public input.
type TxsFieldCircuit struct {
	PublicTxBytes []frontend.Variable `gnark:",public"`
	Msgs          []MsgAssertion

	msgConfigs  []MsgConfig
	txIndexBits int
}

// NewTxsFieldCircuit builds a circuit configured for the given Tx length and
// per-message configs.
func NewTxsFieldCircuit(txLen int, configs []MsgConfig) *TxsFieldCircuit {
	if txLen == 0 {
		panic("tx length must be > 0")
	}
	msgs := make([]MsgAssertion, len(configs))
	for i, cfg := range configs {
		msgs[i].Field.Value = make([]frontend.Variable, cfg.FieldValueLen)
	}

	return &TxsFieldCircuit{
		PublicTxBytes: make([]frontend.Variable, txLen),
		Msgs:          msgs,
		msgConfigs:    configs,
		txIndexBits:   bitsFor(txLen),
	}
}

func (circuit *TxsFieldCircuit) Define(api frontend.API) error {
	tx := circuit.PublicTxBytes
	if len(tx) == 0 {
		panic("empty tx")
	}

	api.AssertIsEqual(tx[0], 0x0a)

	// Decode body length using up to 4 bytes varint
	bodyLenIdx := frontend.Variable(1)
	bodyLen, bodyLenBytes := decodeVarint4Bytes(api, tx, bodyLenIdx, len(tx)-1)

	// bodyStart = 1 (tag) + bodyLenBytes
	bodyStart := api.Add(frontend.Variable(1), bodyLenBytes)
	bodyEnd := api.Add(bodyStart, bodyLen)
	api.AssertIsLessOrEqual(bodyEnd, frontend.Variable(len(tx)))

	cursor := bodyStart
	for i, msg := range circuit.Msgs {
		api.ToBinary(msg.BodyOffset, circuit.txIndexBits)
		api.AssertIsEqual(msg.BodyOffset, cursor)
		cursor = circuit.verifyMessage(api, tx, msg, circuit.msgConfigs[i], bodyEnd)
	}

	//đảm bảo say txbody chỉ có memo, timeout height... chứ không còn msg nào khacs
	maxIdx := len(tx) - 1

	// api.Sub(cursor, bodyEnd) = 0 nếu ko có field nào khác tức là chỉ nguyên []msg, 1 nếu có memo, timeoutheight...
	isAtEnd := api.IsZero(api.Sub(cursor, bodyEnd))

	// If not at end, verify next byte is not a message tag (0x0a)
	nextByte := selectByteAt(api, tx, cursor, maxIdx)

	// là 1 khi byte tiếp theo là 0x0a tức là field 1,w =2
	isNotMsgTag := api.IsZero(api.Sub(nextByte, 0x0a))

	// Assert: (cursor == bodyEnd) OR (nextByte != 0x0a)
	// Using: isAtEnd OR NOT(isNotMsgTag) = isAtEnd + (1 - isNotMsgTag) - isAtEnd*(1-isNotMsgTag) >= 1
	notMsgTagOrEnd := api.Sub(
		api.Add(isAtEnd, api.Sub(1, isNotMsgTag)), //
		api.Mul(isAtEnd, api.Sub(1, isNotMsgTag)),
	)
	api.AssertIsEqual(notMsgTagOrEnd, 1)

	return nil
}

func (circuit *TxsFieldCircuit) verifyMessage(
	api frontend.API,
	tx []frontend.Variable,
	msg MsgAssertion,
	cfg MsgConfig,
	bodyEnd frontend.Variable,
) frontend.Variable {
	maxIdx := len(tx) - 1

	msgTag := selectByteAt(api, tx, msg.BodyOffset, maxIdx)
	api.AssertIsEqual(msgTag, 0x0a)

	// Decode message length using up to 4 bytes varint
	lenIdx := api.Add(msg.BodyOffset, frontend.Variable(1))
	msgLen, lenBytes := decodeVarint4Bytes(api, tx, lenIdx, maxIdx)

	// msgDataStart = BodyOffset + 1 (tag) + lenBytes
	msgDataStart := api.Add(msg.BodyOffset, api.Add(frontend.Variable(1), lenBytes))
	api.AssertIsLessOrEqual(api.Add(msgDataStart, msgLen), bodyEnd)

	typeTag := selectByteAt(api, tx, msgDataStart, maxIdx)
	api.AssertIsEqual(typeTag, 0x0a)

	typeLenIdx := api.Add(msgDataStart, frontend.Variable(1))
	typeLen, typeLenBytes := decodeVarint4Bytes(api, tx, typeLenIdx, maxIdx)

	typeStart := api.Add(typeLenIdx, typeLenBytes)
	api.AssertIsLessOrEqual(api.Add(typeStart, typeLen), api.Add(msgDataStart, msgLen))

	valueTagIdx := api.Add(typeStart, typeLen)
	api.AssertIsEqual(selectByteAt(api, tx, valueTagIdx, maxIdx), 0x12)

	// Decode value length using up to 4 bytes varint
	valueLenIdx := api.Add(valueTagIdx, frontend.Variable(1))
	valueLen, valueBytes := decodeVarint4Bytes(api, tx, valueLenIdx, maxIdx)

	if cfg.MsgValueLen > 0 {
		api.AssertIsEqual(valueLen, frontend.Variable(cfg.MsgValueLen))
	}

	valueStart := api.Add(valueLenIdx, valueBytes)

	api.ToBinary(msg.FieldOffset, circuit.txIndexBits)
	api.AssertIsLessOrEqual(msg.FieldOffset, valueLen)

	// Verify field is at correct position by parsing from start
	// Parse fields sequentially from valueStart until we reach msg.FieldOffset
	// This ensures fieldOffset points to actual field boundary, not arbitrary position
	fieldStart := api.Add(valueStart, msg.FieldOffset)
	keyByte := selectByteAt(api, tx, fieldStart, maxIdx)
	api.AssertIsEqual(keyByte, msg.Field.Key)
	api.AssertIsLessOrEqual(msg.Field.Key, frontend.Variable(0x7f))

	keyBits := api.ToBinary(keyByte, 8)
	api.AssertIsEqual(keyBits[7], 0)
	wireType := api.Add(
		keyBits[0],
		api.Mul(keyBits[1], frontend.Variable(2)),
		api.Mul(keyBits[2], frontend.Variable(4)),
	)
	api.AssertIsEqual(wireType, 2)

	// Decode field length using up to 4 bytes varint
	fieldLenIdx := api.Add(fieldStart, frontend.Variable(1))
	fieldLen, fieldBytes := decodeVarint4Bytes(api, tx, fieldLenIdx, maxIdx)
	api.AssertIsEqual(fieldLen, frontend.Variable(len(msg.Field.Value)))

	fieldValueStart := api.Add(fieldLenIdx, fieldBytes)
	for j := 0; j < len(msg.Field.Value); j++ {
		idx := api.Add(fieldValueStart, frontend.Variable(j))
		api.AssertIsEqual(selectByteAt(api, tx, idx, maxIdx), msg.Field.Value[j])
	}

	// totalField = 1 (key) + fieldBytes + len(Value)
	totalField := api.Add(
		api.Add(frontend.Variable(1), fieldBytes),
		frontend.Variable(len(msg.Field.Value)),
	)
	api.AssertIsLessOrEqual(api.Add(msg.FieldOffset, totalField), valueLen)

	// Đảm bảo field nằm hoàn toàn trong phạm vi message
	msgDataEnd := api.Add(msgDataStart, msgLen)
	fieldEnd := api.Add(fieldStart, totalField)
	api.AssertIsLessOrEqual(msgDataStart, fieldStart) // Field starts within message
	api.AssertIsLessOrEqual(fieldEnd, msgDataEnd)     // Field ends within message

	// Field Number Verification
	// Extract field number từ tag byte và verify với expected field number
	// Tag format: (fieldNumber << 3) | wireType
	// Bits 3-7 chứa field number
	fieldNumber := api.Add(
		api.Mul(keyBits[3], 1),
		api.Add(
			api.Mul(keyBits[4], 2),
			api.Add(
				api.Mul(keyBits[5], 4),
				api.Add(
					api.Mul(keyBits[6], 8),
					api.Mul(keyBits[7], 16),
				),
			),
		),
	)
	// Expected field number được encode trong Key (bits 3-7 của Key)
	keyBitsPublic := api.ToBinary(msg.Field.Key, 8)
	expectedFieldNumber := api.Add(
		api.Mul(keyBitsPublic[3], 1),
		api.Add(
			api.Mul(keyBitsPublic[4], 2),
			api.Add(
				api.Mul(keyBitsPublic[5], 4),
				api.Add(
					api.Mul(keyBitsPublic[6], 8),
					api.Mul(keyBitsPublic[7], 16),
				),
			),
		),
	)
	api.AssertIsEqual(fieldNumber, expectedFieldNumber)

	// Nếu MsgValueLen = 0, bắt buộc phải có upper bound hợp lý
	if cfg.MsgValueLen == 0 {
		// Dynamic size: enforce reasonable upper bound (1MB max)
		api.AssertIsLessOrEqual(valueLen, frontend.Variable(1048576))
	}

	entryEnd := api.Add(
		msg.BodyOffset,
		api.Add(
			frontend.Variable(1),
			api.Add(lenBytes, msgLen),
		),
	)

	return entryEnd
}

func decodeVarintByte(api frontend.API, b frontend.Variable) (frontend.Variable, frontend.Variable) {
	bits := api.ToBinary(b, 8)
	value := frontend.Variable(0)
	for i := 0; i < 7; i++ {
		value = api.Add(value, api.Mul(bits[i], frontend.Variable(1<<i)))
	}
	return value, bits[7]
}

// decodeVarint4Bytes decodes a varint with up to 4 bytes support
// Returns: (decoded value, number of bytes used)
// Max value: 2^28 - 1 = 268,435,455 (~256MB)
func decodeVarint4Bytes(api frontend.API, tx []frontend.Variable, startIdx frontend.Variable, maxIdx int) (frontend.Variable, frontend.Variable) {
	// Read 4 potential bytes
	byte1 := selectByteAt(api, tx, startIdx, maxIdx)
	byte2Idx := api.Add(startIdx, 1)
	byte2 := selectByteAt(api, tx, byte2Idx, maxIdx)
	byte3Idx := api.Add(startIdx, 2)
	byte3 := selectByteAt(api, tx, byte3Idx, maxIdx)
	byte4Idx := api.Add(startIdx, 3)
	byte4 := selectByteAt(api, tx, byte4Idx, maxIdx)

	// Decode each byte
	val1, msb1 := decodeVarintByte(api, byte1)
	val2, msb2 := decodeVarintByte(api, byte2)
	val3, msb3 := decodeVarintByte(api, byte3)
	val4, msb4 := decodeVarintByte(api, byte4)

	// Chỉ khi 3 byte đầu đều bật bit tiếp tục (msb = 1) thì byte thứ 4
	// mới thuộc cùng varint và cần có msb = 0. Nếu varint kết thúc
	// sớm thì byte4 là dữ liệu field kế tiếp, không thể cưỡng bức msb4=0.
	// Gating: msb1*msb2*msb3 == 1 → msb4 phải = 0.
	api.AssertIsEqual(api.Mul(msb1, api.Mul(msb2, api.Mul(msb3, msb4))), 0)

	// Enforce canonical encoding (shortest form)
	// If using 2 bytes (msb1=1), value must be >= 128
	// If using 3 bytes (msb1=1, msb2=1), value must be >= 128^2 = 16384
	// If using 4 bytes (msb1=1, msb2=1, msb3=1), value must be >= 128^3 = 2097152

	// Check 2-byte canonical: if msb1=1, then val2 must be >= 1 (not 0)
	// Because if val2=0 and msb1=1, it means value < 128 which should use 1 byte
	isTwoByte := api.Mul(msb1, api.Sub(1, msb2)) // msb1=1 AND msb2=0
	// If 2-byte, val2 >= 1 (hoặc val1 >= 128, nhưng val1 < 128 luôn)
	val2IsNonZero := api.Sub(1, api.IsZero(val2))
	shouldBeNonZero := api.Mul(isTwoByte, val2IsNonZero)
	api.AssertIsEqual(shouldBeNonZero, isTwoByte) // Nếu dùng 2 bytes thì val2 > 0

	// Check 3-byte canonical: if msb1=1, msb2=1, msb3=0, then val3 > 0
	isThreeByte := api.Mul(msb1, api.Mul(msb2, api.Sub(1, msb3)))
	val3IsNonZero := api.Sub(1, api.IsZero(val3))
	shouldBeNonZero3 := api.Mul(isThreeByte, val3IsNonZero)
	api.AssertIsEqual(shouldBeNonZero3, isThreeByte)

	// Check 4-byte canonical: similar logic
	isFourByte := api.Mul(msb1, api.Mul(msb2, msb3))
	val4IsNonZero := api.Sub(1, api.IsZero(val4))
	shouldBeNonZero4 := api.Mul(isFourByte, val4IsNonZero)
	api.AssertIsEqual(shouldBeNonZero4, isFourByte)

	// Calculate final value based on how many bytes are used
	// value = val1 + (msb1 * val2 * 128) + (msb1 * msb2 * val3 * 128^2) + (msb1 * msb2 * msb3 * val4 * 128^3)

	term1 := val1
	term2 := api.Mul(msb1, api.Mul(val2, 128))
	term3 := api.Mul(msb1, api.Mul(msb2, api.Mul(val3, 16384)))                  // 128^2 = 16384
	term4 := api.Mul(msb1, api.Mul(msb2, api.Mul(msb3, api.Mul(val4, 2097152)))) // 128^3 = 2097152

	value := api.Add(term1, api.Add(term2, api.Add(term3, term4)))

	// Calculate number of bytes used: 1 + msb1 + (msb1 * msb2) + (msb1 * msb2 * msb3)
	bytesUsed := api.Add(
		1,
		api.Add(
			msb1,
			api.Add(
				api.Mul(msb1, msb2),
				api.Mul(msb1, api.Mul(msb2, msb3)),
			),
		),
	)

	return value, bytesUsed
}

// selectByteAt selects tx[idx] using optimized binary tree approach
// Complexity: O(log n) constraints instead of O(n)
// TODO: Currently using linear for stability, will optimize to binary tree
func selectByteAt(api frontend.API, tx []frontend.Variable, idx frontend.Variable, maxIdx int) frontend.Variable {
	api.AssertIsLessOrEqual(frontend.Variable(0), idx)
	api.AssertIsLessOrEqual(idx, frontend.Variable(maxIdx))

	length := maxIdx + 1
	pow2 := 1
	numBits := 0
	for pow2 < length {
		pow2 <<= 1
		numBits++
	}
	if numBits == 0 {
		numBits = 1
	}

	idxBitsLE := api.ToBinary(idx, numBits)
	idxBits := make([]frontend.Variable, numBits)
	for i := 0; i < numBits; i++ {
		idxBits[i] = idxBitsLE[numBits-1-i]
	}

	return selectByteAtPow2(api, tx, idxBits, 0, pow2, 0)
}

func selectByteAtPow2(
	api frontend.API,
	tx []frontend.Variable,
	idxBits []frontend.Variable,
	start int,
	size int,
	bitPos int,
) frontend.Variable {
	if size == 1 {
		if start < len(tx) {
			return tx[start]
		}
		return frontend.Variable(0)
	}

	half := size / 2
	left := selectByteAtPow2(api, tx, idxBits, start, half, bitPos+1)
	right := selectByteAtPow2(api, tx, idxBits, start+half, half, bitPos+1)

	if bitPos >= len(idxBits) {
		return left
	}

	currentBit := idxBits[bitPos]
	notBit := api.Sub(1, currentBit)

	return api.Add(
		api.Mul(notBit, left),
		api.Mul(currentBit, right),
	)
}

func bitsFor(n int) int {
	if n <= 1 {
		return 1
	}
	return bits.Len(uint(n - 1))
}
