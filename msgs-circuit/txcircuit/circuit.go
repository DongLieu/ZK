package txcircuit

import (
	"math/bits"

	"github.com/consensys/gnark/frontend"
)

// FieldPublic mô tả một field length-delimited trong Msg (protobuf) được đưa ra
// làm public input: key (varint field number | wire type) và value bytes.
type FieldPublic struct {
	Key   frontend.Variable   `gnark:",public"`
	Value []frontend.Variable `gnark:",public"`
}

// TxFieldCircuit chứng minh TxBytes chứa Msg có TypeURL (ở dạng secret) và trong
// Msg.Value tồn tại field (key/value) đúng như public input.
type TxFieldCircuit struct {
	TxBytes       []frontend.Variable `gnark:",secret"`
	PublicTxBytes []frontend.Variable `gnark:",public"`
	MsgType       []frontend.Variable `gnark:",secret"`
	FieldOffset   frontend.Variable   `gnark:",secret"`
	Field         FieldPublic

	msgValueLen int
	txIndexBits int
}

const (
	txRawBodyLenVarintBytes = 2
	bodyMsgLenVarintBytes   = 2
)

func (circuit *TxFieldCircuit) Define(api frontend.API) error {
	tx := circuit.TxBytes
	if len(tx) == 0 {
		panic("empty tx bytes")
	}

	// Ràng buộc TxBytes witness == public TxBytes.
	for i := range tx {
		api.AssertIsEqual(tx[i], circuit.PublicTxBytes[i])
	}

	// TxRaw.BodyBytes field number 1 => tag 0x0a.
	api.AssertIsEqual(tx[0], 0x0a)

	bodyStart := 1 + txRawBodyLenVarintBytes
	api.AssertIsEqual(tx[bodyStart], 0x0a)

	// TxBody.Messages[0] => Any{TypeURL, Value}
	typeTagIdx := bodyStart + 1 + bodyMsgLenVarintBytes
	api.AssertIsEqual(tx[typeTagIdx], 0x0a)

	typeLen := len(circuit.MsgType)
	typeLenIdx := typeTagIdx + 1
	api.AssertIsEqual(tx[typeLenIdx], typeLen)

	typeStart := typeLenIdx + 1
	for i := 0; i < typeLen; i++ {
		api.AssertIsEqual(tx[typeStart+i], circuit.MsgType[i])
	}

	// Any.Value tag.
	valueTagIdx := typeStart + typeLen
	api.AssertIsEqual(tx[valueTagIdx], 0x12)

	// Decode chiều dài Msg (Any.Value) bằng varint (tối đa 2 bytes).
	valueLenIdx := valueTagIdx + 1
	valueLenLow, firstMSB := decodeVarintByte(api, tx[valueLenIdx])
	valueLenHigh, highMSB := decodeVarintByte(api, tx[valueLenIdx+1])
	api.AssertIsEqual(api.Mul(firstMSB, highMSB), 0)

	valueLen := api.Add(
		valueLenLow,
		api.Mul(firstMSB, api.Mul(valueLenHigh, frontend.Variable(128))),
	)

	if circuit.msgValueLen > 0 {
		api.AssertIsEqual(valueLen, frontend.Variable(circuit.msgValueLen))
	}

	// đảm bảo Msg.Value không vượt quá TxBytes length.
	valueStart := api.Add(frontend.Variable(valueLenIdx+1), firstMSB)
	api.AssertIsLessOrEqual(
		api.Add(valueStart, valueLen),
		frontend.Variable(len(tx)),
	)

	// Ràng buộc FieldOffset là số nguyên trong [0, valueLen).
	api.ToBinary(circuit.FieldOffset, circuit.txIndexBits)
	api.AssertIsLessOrEqual(
		api.Add(circuit.FieldOffset, frontend.Variable(1)),
		valueLen,
	)

	fieldStart := api.Add(valueStart, circuit.FieldOffset)
	maxIdx := len(tx) - 1

	keyByte := selectByteAt(api, tx, fieldStart, maxIdx)
	api.AssertIsEqual(keyByte, circuit.Field.Key)
	api.AssertIsLessOrEqual(circuit.Field.Key, frontend.Variable(0x7f))

	// Field key luôn 1 byte => bit 7 phải = 0, wire-type = 2 (length-delimited).
	keyBits := api.ToBinary(keyByte, 8)
	api.AssertIsEqual(keyBits[7], 0)
	wireType := api.Add(
		keyBits[0],
		api.Mul(keyBits[1], frontend.Variable(2)),
		api.Mul(keyBits[2], frontend.Variable(4)),
	)
	api.AssertIsEqual(wireType, 2)

	// Decode varint length của field value.
	lenIdx := api.Add(fieldStart, frontend.Variable(1))
	lenByte := selectByteAt(api, tx, lenIdx, maxIdx)
	lenLow, lenMSB := decodeVarintByte(api, lenByte)

	lenHighIdx := api.Add(lenIdx, frontend.Variable(1))
	lenHighByte := selectByteAt(api, tx, lenHighIdx, maxIdx)
	lenHigh, lenHighMSB := decodeVarintByte(api, lenHighByte)
	api.AssertIsEqual(api.Mul(lenMSB, lenHighMSB), 0)

	valueLenVar := api.Add(
		lenLow,
		api.Mul(lenMSB, api.Mul(lenHigh, frontend.Variable(128))),
	)

	expectedValueLen := frontend.Variable(len(circuit.Field.Value))
	api.AssertIsEqual(valueLenVar, expectedValueLen)

	lenBytes := api.Add(frontend.Variable(1), lenMSB)
	fieldTotalLen := api.Add(
		api.Add(frontend.Variable(1), lenBytes),
		expectedValueLen,
	)

	// Offset + field length phải nằm trong Msg.Value.
	api.AssertIsLessOrEqual(
		api.Add(circuit.FieldOffset, fieldTotalLen),
		valueLen,
	)

	fieldValueStart := api.Add(lenIdx, api.Add(frontend.Variable(1), lenMSB))
	for i := 0; i < len(circuit.Field.Value); i++ {
		idx := api.Add(fieldValueStart, frontend.Variable(i))
		valByte := selectByteAt(api, tx, idx, maxIdx)
		api.AssertIsEqual(valByte, circuit.Field.Value[i])
	}

	return nil
}

func decodeVarintByte(api frontend.API, b frontend.Variable) (frontend.Variable, frontend.Variable) {
	bits := api.ToBinary(b, 8)
	value := frontend.Variable(0)
	for i := 0; i < 7; i++ {
		value = api.Add(value, api.Mul(bits[i], frontend.Variable(1<<i)))
	}
	return value, bits[7]
}

func selectByteAt(api frontend.API, tx []frontend.Variable, idx frontend.Variable, maxIdx int) frontend.Variable {
	api.AssertIsLessOrEqual(frontend.Variable(0), idx)
	api.AssertIsLessOrEqual(idx, frontend.Variable(maxIdx))

	result := frontend.Variable(0)
	for pos := 0; pos <= maxIdx; pos++ {
		isPos := api.IsZero(api.Sub(idx, frontend.Variable(pos)))
		result = api.Add(result, api.Mul(isPos, tx[pos]))
	}
	return result
}

// NewTxFieldCircuit cấu hình circuit với các kích thước cố định.
func NewTxFieldCircuit(txBytesLen, msgTypeLen, fieldValueLen, msgValueLen int) *TxFieldCircuit {
	if txBytesLen == 0 {
		panic("txBytesLen must be > 0")
	}

	return &TxFieldCircuit{
		TxBytes:       make([]frontend.Variable, txBytesLen),
		PublicTxBytes: make([]frontend.Variable, txBytesLen),
		MsgType:       make([]frontend.Variable, msgTypeLen),
		Field: FieldPublic{
			Value: make([]frontend.Variable, fieldValueLen),
		},
		msgValueLen: msgValueLen,
		txIndexBits: bitsFor(txBytesLen),
	}
}

func bitsFor(n int) int {
	if n <= 1 {
		return 1
	}
	return bits.Len(uint(n - 1))
}
