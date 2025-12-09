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
	MsgType     []frontend.Variable `gnark:",secret"`
	Field       FieldPublic
	FieldOffset frontend.Variable `gnark:",secret"`
	BodyOffset  frontend.Variable `gnark:",secret"`
}

// MsgConfig định nghĩa kích thước cố định cho từng message assertion.
type MsgConfig struct {
	MsgTypeLen    int
	FieldValueLen int
	MsgValueLen   int
}

// TxsFieldCircuit chứng minh TxBytes chứa nhiều Msg (có thể >1) và mỗi Msg
// có Field length-delimited khớp public input.
type TxsFieldCircuit struct {
	TxBytes       []frontend.Variable `gnark:",secret"`
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
		msgs[i].MsgType = make([]frontend.Variable, cfg.MsgTypeLen)
		msgs[i].Field.Value = make([]frontend.Variable, cfg.FieldValueLen)
	}

	return &TxsFieldCircuit{
		TxBytes:       make([]frontend.Variable, txLen),
		PublicTxBytes: make([]frontend.Variable, txLen),
		Msgs:          msgs,
		msgConfigs:    configs,
		txIndexBits:   bitsFor(txLen),
	}
}

func (circuit *TxsFieldCircuit) Define(api frontend.API) error {
	tx := circuit.TxBytes
	if len(tx) == 0 {
		panic("empty tx")
	}

	for i := range tx {
		api.AssertIsEqual(tx[i], circuit.PublicTxBytes[i])
	}

	api.AssertIsEqual(tx[0], 0x0a)

	bodyLenIdx := frontend.Variable(1)
	bodyLenLow, bodyLenMSB := decodeVarintByte(api, tx[1])
	bodyLenHigh, bodyLenHighMSB := decodeVarintByte(api, tx[2])
	api.AssertIsEqual(api.Mul(bodyLenMSB, bodyLenHighMSB), 0)
	bodyLen := api.Add(bodyLenLow, api.Mul(bodyLenMSB, api.Mul(bodyLenHigh, frontend.Variable(128))))

	bodyStart := api.Add(bodyLenIdx, api.Add(frontend.Variable(1), bodyLenMSB))
	bodyEnd := api.Add(bodyStart, bodyLen)
	api.AssertIsLessOrEqual(bodyEnd, frontend.Variable(len(tx)))

	prevOffset := bodyStart
	for i, msg := range circuit.Msgs {
		api.ToBinary(msg.BodyOffset, circuit.txIndexBits)
		api.AssertIsLessOrEqual(prevOffset, msg.BodyOffset)
		api.AssertIsLessOrEqual(msg.BodyOffset, api.Sub(bodyEnd, frontend.Variable(1)))

		circuit.verifyMessage(api, tx, msg, circuit.msgConfigs[i], bodyStart, bodyEnd)

		prevOffset = api.Add(msg.BodyOffset, frontend.Variable(1))
	}

	return nil
}

func (circuit *TxsFieldCircuit) verifyMessage(
	api frontend.API,
	tx []frontend.Variable,
	msg MsgAssertion,
	cfg MsgConfig,
	bodyStart frontend.Variable,
	bodyEnd frontend.Variable,
) {
	maxIdx := len(tx) - 1

	msgTag := selectByteAt(api, tx, msg.BodyOffset, maxIdx)
	api.AssertIsEqual(msgTag, 0x0a)

	lenIdx := api.Add(msg.BodyOffset, frontend.Variable(1))
	lenByte := selectByteAt(api, tx, lenIdx, maxIdx)
	lenLow, lenMSB := decodeVarintByte(api, lenByte)

	lenHighIdx := api.Add(lenIdx, frontend.Variable(1))
	lenHighByte := selectByteAt(api, tx, lenHighIdx, maxIdx)
	lenHigh, lenHighMSB := decodeVarintByte(api, lenHighByte)
	api.AssertIsEqual(api.Mul(lenMSB, lenHighMSB), 0)

	msgLen := api.Add(lenLow, api.Mul(lenMSB, api.Mul(lenHigh, frontend.Variable(128))))
	lenBytes := api.Add(frontend.Variable(1), lenMSB)

	msgDataStart := api.Add(msg.BodyOffset, api.Add(frontend.Variable(1), lenBytes))
	api.AssertIsLessOrEqual(api.Add(msgDataStart, msgLen), bodyEnd)

	typeTag := selectByteAt(api, tx, msgDataStart, maxIdx)
	api.AssertIsEqual(typeTag, 0x0a)

	typeLenIdx := api.Add(msgDataStart, frontend.Variable(1))
	typeLenByte := selectByteAt(api, tx, typeLenIdx, maxIdx)
	api.AssertIsEqual(typeLenByte, frontend.Variable(len(msg.MsgType)))

	typeStart := api.Add(typeLenIdx, frontend.Variable(1))
	for j := 0; j < len(msg.MsgType); j++ {
		idx := api.Add(typeStart, frontend.Variable(j))
		api.AssertIsEqual(selectByteAt(api, tx, idx, maxIdx), msg.MsgType[j])
	}

	valueTagIdx := api.Add(typeStart, frontend.Variable(len(msg.MsgType)))
	api.AssertIsEqual(selectByteAt(api, tx, valueTagIdx, maxIdx), 0x12)

	valueLenIdx := api.Add(valueTagIdx, frontend.Variable(1))
	valueLenByte := selectByteAt(api, tx, valueLenIdx, maxIdx)
	valueLenLow, valueMSB := decodeVarintByte(api, valueLenByte)

	valueHighIdx := api.Add(valueLenIdx, frontend.Variable(1))
	valueHighByte := selectByteAt(api, tx, valueHighIdx, maxIdx)
	valueLenHigh, valueHighMSB := decodeVarintByte(api, valueHighByte)
	api.AssertIsEqual(api.Mul(valueMSB, valueHighMSB), 0)

	valueLen := api.Add(valueLenLow, api.Mul(valueMSB, api.Mul(valueLenHigh, frontend.Variable(128))))
	if cfg.MsgValueLen > 0 {
		api.AssertIsEqual(valueLen, frontend.Variable(cfg.MsgValueLen))
	}

	valueBytes := api.Add(frontend.Variable(1), valueMSB)
	valueStart := api.Add(valueLenIdx, valueBytes)

	api.ToBinary(msg.FieldOffset, circuit.txIndexBits)
	api.AssertIsLessOrEqual(msg.FieldOffset, valueLen)

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

	fieldLenIdx := api.Add(fieldStart, frontend.Variable(1))
	fieldLenByte := selectByteAt(api, tx, fieldLenIdx, maxIdx)
	fieldLenLow, fieldLenMSB := decodeVarintByte(api, fieldLenByte)

	fieldLenHighIdx := api.Add(fieldLenIdx, frontend.Variable(1))
	fieldLenHighByte := selectByteAt(api, tx, fieldLenHighIdx, maxIdx)
	fieldLenHigh, fieldLenHighMSB := decodeVarintByte(api, fieldLenHighByte)
	api.AssertIsEqual(api.Mul(fieldLenMSB, fieldLenHighMSB), 0)

	fieldLen := api.Add(fieldLenLow, api.Mul(fieldLenMSB, api.Mul(fieldLenHigh, frontend.Variable(128))))
	api.AssertIsEqual(fieldLen, frontend.Variable(len(msg.Field.Value)))

	fieldBytes := api.Add(frontend.Variable(1), fieldLenMSB)
	fieldValueStart := api.Add(fieldLenIdx, fieldBytes)
	for j := 0; j < len(msg.Field.Value); j++ {
		idx := api.Add(fieldValueStart, frontend.Variable(j))
		api.AssertIsEqual(selectByteAt(api, tx, idx, maxIdx), msg.Field.Value[j])
	}

	totalField := api.Add(
		api.Add(frontend.Variable(2), fieldLenMSB),
		frontend.Variable(len(msg.Field.Value)),
	)
	api.AssertIsLessOrEqual(api.Add(msg.FieldOffset, totalField), valueLen)
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

func bitsFor(n int) int {
	if n <= 1 {
		return 1
	}
	return bits.Len(uint(n - 1))
}
