package txcircuit

import "github.com/consensys/gnark/frontend"

// CoinPublic gom denom + amount (dạng chuỗi bytes) làm public input.
type CoinPublic struct {
	Denom  []frontend.Variable `gnark:",public"`
	Amount []frontend.Variable `gnark:",public"`
}

// TxDecodeCircuit chứng minh TxBytes chứa MsgSend với TypeURL, địa chỉ gửi
// và coin khớp public input.
type TxDecodeCircuit struct {
	TxBytes       []frontend.Variable `gnark:",secret"`
	PublicTxBytes []frontend.Variable `gnark:",public"`
	MsgType       []frontend.Variable `gnark:",secret"`
	ExpectedFrom  []frontend.Variable `gnark:",public"`
	ExpectedCoins CoinPublic
}

// Define mô tả constraint dựa trên cấu trúc TxRaw do txcodec.Encode sinh ra.
const (
	txRawBodyLenVarintBytes = 2
	bodyMsgLenVarintBytes   = 2
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

	// chiều dài TypeURL phải đúng với witness secret
	typeLen := len(circuit.MsgType)
	typeLenIdx := typeTagIdx + 1
	api.AssertIsEqual(tx[typeLenIdx], typeLen)

	// ràng buộc từng byte TypeURL
	typeStart := typeLenIdx + 1
	for i := 0; i < typeLen; i++ {
		api.AssertIsEqual(tx[typeStart+i], circuit.MsgType[i])
	}

	// tag Any.Value và chiều dài
	valueTagIdx := typeStart + typeLen
	api.AssertIsEqual(tx[valueTagIdx], 0x12)

	valueLenIdx := valueTagIdx + 1
	valueLenLow, firstMSB := decodeVarintByte(api, tx[valueLenIdx])
	valueLenHigh, highMSB := decodeVarintByte(api, tx[valueLenIdx+1])
	api.AssertIsEqual(highMSB, 0)

	valueLen := api.Add(valueLenLow, api.Mul(firstMSB, api.Mul(valueLenHigh, frontend.Variable(128))))
	api.AssertIsLessOrEqual(frontend.Variable(len(circuit.ExpectedFrom)+2), valueLen)

	valueStartOne := valueLenIdx + 1
	valueStartTwo := valueLenIdx + 2

	selectByte := func(idxOne, idxTwo int) frontend.Variable {
		return api.Select(firstMSB, tx[idxTwo], tx[idxOne])
	}

	api.AssertIsEqual(selectByte(valueStartOne, valueStartTwo), 0x0a)

	fromLen := len(circuit.ExpectedFrom)
	api.AssertIsEqual(selectByte(valueStartOne+1, valueStartTwo+1), frontend.Variable(fromLen))

	fromStartOne := valueStartOne + 2
	fromStartTwo := valueStartTwo + 2
	for i := 0; i < fromLen; i++ {
		api.AssertIsEqual(
			selectByte(fromStartOne+i, fromStartTwo+i),
			circuit.ExpectedFrom[i],
		)
	}

	toTagIdxOne := fromStartOne + fromLen
	toTagIdxTwo := fromStartTwo + fromLen
	api.AssertIsEqual(selectByte(toTagIdxOne, toTagIdxTwo), 0x12)
	api.AssertIsEqual(
		selectByte(toTagIdxOne+1, toTagIdxTwo+1),
		frontend.Variable(fromLen),
	)

	toStartOne := toTagIdxOne + 2
	toStartTwo := toTagIdxTwo + 2

	coinTagIdxOne := toStartOne + fromLen
	coinTagIdxTwo := toStartTwo + fromLen
	api.AssertIsEqual(selectByte(coinTagIdxOne, coinTagIdxTwo), 0x1a)

	coinLenIdxOne := coinTagIdxOne + 1
	coinLenIdxTwo := coinTagIdxTwo + 1
	coinLenLow, coinMSB := decodeVarintByte(api, selectByte(coinLenIdxOne, coinLenIdxTwo))
	coinLenHigh, coinHighMSB := decodeVarintByte(api, selectByte(coinLenIdxOne+1, coinLenIdxTwo+1))
	api.AssertIsEqual(coinHighMSB, 0)
	coinLen := api.Add(coinLenLow, api.Mul(coinMSB, api.Mul(coinLenHigh, frontend.Variable(128))))

	coinStartOne := coinLenIdxOne + 1
	coinStartTwo := coinLenIdxTwo + 2

	api.AssertIsEqual(selectByte(coinStartOne, coinStartTwo), 0x0a)
	denomLen := len(circuit.ExpectedCoins.Denom)
	api.AssertIsEqual(selectByte(coinStartOne+1, coinStartTwo+1), frontend.Variable(denomLen))

	denomStartOne := coinStartOne + 2
	denomStartTwo := coinStartTwo + 2
	for i := 0; i < denomLen; i++ {
		api.AssertIsEqual(
			selectByte(denomStartOne+i, denomStartTwo+i),
			circuit.ExpectedCoins.Denom[i],
		)
	}

	amountTagIdxOne := denomStartOne + denomLen
	amountTagIdxTwo := denomStartTwo + denomLen
	api.AssertIsEqual(selectByte(amountTagIdxOne, amountTagIdxTwo), 0x12)

	amountLenVal := selectByte(amountTagIdxOne+1, amountTagIdxTwo+1)
	amountLen, amountMSB := decodeVarintByte(api, amountLenVal)
	api.AssertIsEqual(amountMSB, 0)
	api.AssertIsEqual(amountLen, frontend.Variable(len(circuit.ExpectedCoins.Amount)))

	amountStartOne := amountTagIdxOne + 2
	amountStartTwo := amountTagIdxTwo + 2
	for i := 0; i < len(circuit.ExpectedCoins.Amount); i++ {
		api.AssertIsEqual(
			selectByte(amountStartOne+i, amountStartTwo+i),
			circuit.ExpectedCoins.Amount[i],
		)
	}

	api.AssertIsLessOrEqual(
		frontend.Variable(len(circuit.ExpectedCoins.Amount)+denomLen+4),
		coinLen,
	)

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

// NewTxDecodeCircuit builds a circuit with the configured array lengths.
func NewTxDecodeCircuit(txBytesLen, msgTypeLen, addrLen, amountLen, denomLen int) *TxDecodeCircuit {
	return &TxDecodeCircuit{
		TxBytes:       make([]frontend.Variable, txBytesLen),
		PublicTxBytes: make([]frontend.Variable, txBytesLen),
		MsgType:       make([]frontend.Variable, msgTypeLen),
		ExpectedFrom:  make([]frontend.Variable, addrLen),
		ExpectedCoins: CoinPublic{
			Denom:  make([]frontend.Variable, denomLen),
			Amount: make([]frontend.Variable, amountLen),
		},
	}
}
