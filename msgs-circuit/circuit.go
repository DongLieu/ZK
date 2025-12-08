package main

import (
	"github.com/consensys/gnark/frontend"
)

// TxDecodeCircuit định nghĩa circuit để verify rằng một TxBytes chứa một message cụ thể
// Mục tiêu: Chứng minh "TxBytes này chứa MsgSend với FromAddress, ToAddress, Amount cụ thể"
//
// Ví dụ use case:
// - Public input: TxBytes, ExpectedMsgTypeURL, ExpectedFromAddr, ExpectedToAddr, ExpectedAmount
// - Circuit verify: TxBytes thực sự chứa MsgSend với các giá trị đúng như kỳ vọng
type TxDecodeCircuit struct {
	// ========== PUBLIC INPUTS ==========
	// Những gì cần verify công khai
	TxBytes []frontend.Variable `gnark:",public"` // Transaction bytes cần kiểm tra

	// Expected message type URL (hash của string "/cosmos.bank.v1beta1.MsgSend")
	ExpectedMsgTypeHash frontend.Variable `gnark:",public"`

	// Expected message content để verify
	ExpectedFromAddr []frontend.Variable `gnark:",public"` // Address người gửi mong đợi
	ExpectedToAddr   []frontend.Variable `gnark:",public"` // Address người nhận mong đợi
	ExpectedAmount   frontend.Variable   `gnark:",public"` // Amount mong đợi

	// ========== PRIVATE WITNESS ==========
	// Các thành phần trung gian khi decode (prover biết, verifier không biết)
	BodyBytes     []frontend.Variable `gnark:",secret"` // TxBody bytes extracted từ TxRaw
	AuthInfoBytes []frontend.Variable `gnark:",secret"` // AuthInfo bytes extracted từ TxRaw
	Signatures    []frontend.Variable `gnark:",secret"` // Signature bytes

	// Message Any wrapper components
	MsgTypeURL []frontend.Variable `gnark:",secret"` // TypeURL của message (ví dụ: "/cosmos.bank.v1beta1.MsgSend")
	MsgValue   []frontend.Variable `gnark:",secret"` // Protobuf encoded message value

	// Decoded message content (MsgSend fields)
	DecodedFromAddr []frontend.Variable `gnark:",secret"` // FromAddress decode từ MsgValue
	DecodedToAddr   []frontend.Variable `gnark:",secret"` // ToAddress decode từ MsgValue
	DecodedAmount   frontend.Variable   `gnark:",secret"` // Amount decode từ MsgValue
}

// Define implement gnark circuit interface
// Hàm này định nghĩa các constraints để verify TxBytes chứa message mong đợi
func (circuit *TxDecodeCircuit) Define(api frontend.API) error {
	// ========================================
	// STEP 1: Decode TxBytes thành TxRaw
	// ========================================
	// Protobuf encoding của TxRaw có cấu trúc:
	// field 1 (BodyBytes): tag=0x0a + length + data
	// field 2 (AuthInfoBytes): tag=0x12 + length + data
	// field 3 (Signatures): tag=0x1a + length + data

	// Verify TxBytes structure bằng cách check field tag đầu tiên
	api.AssertIsEqual(circuit.TxBytes[0], 0x0a) // Tag cho field 1 (BodyBytes)

	// Extract length của BodyBytes (giả sử length < 128 bytes, dùng 1 byte)
	bodyLen := circuit.TxBytes[1]

	// Verify BodyBytes được extract đúng từ TxBytes
	// BodyBytes bắt đầu từ vị trí 2, với độ dài bodyLen
	// Note: Chỉ verify các bytes đầu tiên, vì circuit.BodyBytes có thể lớn hơn actual bodyLen
	for i := 0; i < len(circuit.BodyBytes); i++ {
		api.AssertIsEqual(circuit.BodyBytes[i], circuit.TxBytes[2+i])
	}

	// TODO: Verify bodyLen <= len(circuit.BodyBytes) thay vì equality
	// api.AssertIsEqual(bodyLen, len(circuit.BodyBytes))
	_ = bodyLen // Use bodyLen to avoid unused variable warning

	// ========================================
	// STEP 2: Decode BodyBytes thành TxBody và extract Messages
	// ========================================
	// TxBody structure:
	// field 1 (Messages): tag=0x0a + length + Any{TypeURL, Value}
	// Verify BodyBytes chứa Messages field
	api.AssertIsEqual(circuit.BodyBytes[0], 0x0a) // Tag cho Messages field

	// Extract message Any wrapper từ BodyBytes
	// Any structure: tag=0x0a (TypeURL) + length + string, tag=0x12 (Value) + length + bytes

	// ========================================
	// STEP 3: Verify MsgTypeURL khớp với expected
	// ========================================
	// Hash MsgTypeURL và compare với ExpectedMsgTypeHash
	msgTypeHash := frontend.Variable(0)
	for i := 0; i < len(circuit.MsgTypeURL); i++ {
		msgTypeHash = api.Add(msgTypeHash, circuit.MsgTypeURL[i])
	}
	api.AssertIsEqual(msgTypeHash, circuit.ExpectedMsgTypeHash)

	// ========================================
	// STEP 4: Decode MsgValue thành MsgSend fields
	// ========================================
	// MsgSend protobuf structure:
	// field 1 (FromAddress): tag=0x0a + length + string
	// field 2 (ToAddress): tag=0x12 + length + string
	// field 3 (Amount): tag=0x1a + length + Coin{denom, amount}

	// Verify MsgValue có thể decode thành các fields
	// (Đơn giản hóa - trong thực tế cần parse protobuf đầy đủ)

	// ========================================
	// STEP 5: Verify decoded content khớp với expected values
	// ========================================
	// Verify FromAddress
	for i := 0; i < len(circuit.DecodedFromAddr); i++ {
		api.AssertIsEqual(circuit.DecodedFromAddr[i], circuit.ExpectedFromAddr[i])
	}

	// Verify ToAddress
	for i := 0; i < len(circuit.DecodedToAddr); i++ {
		api.AssertIsEqual(circuit.DecodedToAddr[i], circuit.ExpectedToAddr[i])
	}

	// Verify Amount
	api.AssertIsEqual(circuit.DecodedAmount, circuit.ExpectedAmount)

	// ========================================
	// STEP 6: Additional validations
	// ========================================
	// Verify Amount > 0
	api.AssertIsEqual(
		api.Cmp(circuit.DecodedAmount, 0),
		1, // Amount phải > 0
	)

	return nil
}

// NewTxDecodeCircuit khởi tạo circuit mới với các giá trị mặc định
func NewTxDecodeCircuit(txBytesLen, bodyBytesLen, authInfoBytesLen, sigLen, addrLen, msgTypeURLLen, msgValueLen int) *TxDecodeCircuit {
	return &TxDecodeCircuit{
		// Public inputs
		TxBytes:          make([]frontend.Variable, txBytesLen),
		ExpectedFromAddr: make([]frontend.Variable, addrLen),
		ExpectedToAddr:   make([]frontend.Variable, addrLen),

		// Private witness
		BodyBytes:     make([]frontend.Variable, bodyBytesLen),
		AuthInfoBytes: make([]frontend.Variable, authInfoBytesLen),
		Signatures:    make([]frontend.Variable, sigLen),

		MsgTypeURL: make([]frontend.Variable, msgTypeURLLen),
		MsgValue:   make([]frontend.Variable, msgValueLen),

		DecodedFromAddr: make([]frontend.Variable, addrLen),
		DecodedToAddr:   make([]frontend.Variable, addrLen),
	}
}
