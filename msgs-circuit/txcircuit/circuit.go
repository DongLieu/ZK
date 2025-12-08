package txcircuit

import "github.com/consensys/gnark/frontend"

// TxDecodeCircuit chứng minh TxBytes chứa MsgSend với TypeURL và địa chỉ gửi
// khớp public input.
type TxDecodeCircuit struct {
	TxBytes         []frontend.Variable `gnark:",secret"`
	PublicTxBytes   []frontend.Variable `gnark:",public"`
	ExpectedMsgType []frontend.Variable `gnark:",public"`
	ExpectedFrom    []frontend.Variable `gnark:",public"`
}

// Define mô tả constraint dựa trên cấu trúc TxRaw do txcodec.Encode sinh ra.
const (
	txRawBodyLenVarintBytes = 2
	bodyMsgLenVarintBytes   = 2
	anyValueLenVarintBytes  = 1
	expectedValueLen        = 0x70
)

func (circuit *TxDecodeCircuit) Define(api frontend.API) error {
	tx := circuit.TxBytes
	if len(tx) < 70 {
		panic("tx bytes too short for constraints")
	}

	// đảm bảo witness TxBytes == public TxBytes
	for i := range tx {
		api.AssertIsEqual(tx[i], circuit.PublicTxBytes[i])
	}

	// tag field 1 của TxRaw (BodyBytes) = 0x0a
	api.AssertIsEqual(tx[0], 0x0a)

	// vị trí bắt đầu BodyBytes bỏ qua tag + length varint
	bodyStart := 1 + txRawBodyLenVarintBytes
	api.AssertIsEqual(tx[bodyStart], 0x0a)

	// Messages field trong TxBody: chứa Any với TypeURL + Value
	typeTagIdx := bodyStart + 1 + bodyMsgLenVarintBytes
	api.AssertIsEqual(tx[typeTagIdx], 0x0a)

	// chiều dài TypeURL phải đúng với public input
	typeLen := len(circuit.ExpectedMsgType)
	typeLenIdx := typeTagIdx + 1
	api.AssertIsEqual(tx[typeLenIdx], typeLen)

	// ràng buộc từng byte TypeURL
	typeStart := typeLenIdx + 1
	for i := 0; i < typeLen; i++ {
		api.AssertIsEqual(tx[typeStart+i], circuit.ExpectedMsgType[i])
	}

	// tag Any.Value và chiều dài (0x70 với MsgSend encode mặc định)
	valueTagIdx := typeStart + typeLen
	api.AssertIsEqual(tx[valueTagIdx], 0x12)

	valueLenIdx := valueTagIdx + 1
	api.AssertIsEqual(tx[valueLenIdx], expectedValueLen)

	valueStart := valueLenIdx + anyValueLenVarintBytes

	// MsgSend field 1: FromAddress
	api.AssertIsEqual(tx[valueStart], 0x0a)

	fromLen := len(circuit.ExpectedFrom)
	api.AssertIsEqual(tx[valueStart+1], fromLen)

	// so khớp từng byte của địa chỉ gửi
	fromStart := valueStart + 2
	for i := 0; i < fromLen; i++ {
		api.AssertIsEqual(tx[fromStart+i], circuit.ExpectedFrom[i])
	}

	// kiểm tra tag field 2 (ToAddress) nằm sau phần FromAddress
	toTagIdx := fromStart + fromLen
	api.AssertIsEqual(tx[toTagIdx], 0x12)

	return nil
}

// NewTxDecodeCircuit builds a circuit with the configured array lengths.
func NewTxDecodeCircuit(txBytesLen, msgTypeLen, addrLen int) *TxDecodeCircuit {
	return &TxDecodeCircuit{
		TxBytes:         make([]frontend.Variable, txBytesLen),
		PublicTxBytes:   make([]frontend.Variable, txBytesLen),
		ExpectedMsgType: make([]frontend.Variable, msgTypeLen),
		ExpectedFrom:    make([]frontend.Variable, addrLen),
	}
}
