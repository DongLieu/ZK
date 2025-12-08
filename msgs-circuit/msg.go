package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	"cosmossdk.io/x/bank/types"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

func Encode() (txBytes []byte) {
	// ========================================
	// PART 1: Tạo key, sinh địa chỉ và message
	// ========================================

	privKey := secp256k1.GenPrivKey()
	fromAddr := sdk.AccAddress(privKey.PubKey().Address()).String()

	// tạo địa chỉ nhận ngẫu nhiên để minh họa
	receiverPrivKey := secp256k1.GenPrivKey()
	toAddr := sdk.AccAddress(receiverPrivKey.PubKey().Address()).String()

	fmt.Println("========== KEY INFO ==========")
	fmt.Printf("Sender private key (hex): %s\n", hex.EncodeToString(privKey.Bytes()))
	fmt.Printf("Sender address: %s\n", fromAddr)
	fmt.Printf("Receiver address: %s\n", toAddr)
	fmt.Println()

	// Tạo MsgSend
	msg := &types.MsgSend{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount: sdk.NewCoins(
			sdk.NewCoin("uatom", math.NewInt(1000000)), // 1 ATOM = 1,000,000 uatom
		),
	}

	fmt.Println("========== MESSAGE OBJECT ==========")
	fmt.Printf("From: %s\n", msg.FromAddress)
	fmt.Printf("To: %s\n", msg.ToAddress)
	fmt.Printf("Amount: %s\n", msg.Amount)
	fmt.Println()

	// ========================================
	// PART 2: PROTOBUF ENCODING
	// ========================================

	// Setup codec (Protobuf encoder/decoder)
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	protoCodec := codec.NewProtoCodec(interfaceRegistry)

	// Marshal message to protobuf binary
	msgBytes, err := protoCodec.Marshal(msg)
	if err != nil {
		panic(err)
	}

	fmt.Println("========== PROTOBUF ENCODING ==========")
	fmt.Printf("Binary (hex): %s\n", hex.EncodeToString(msgBytes))
	fmt.Printf("Binary (base64): %s\n", base64.StdEncoding.EncodeToString(msgBytes))
	fmt.Printf("Size: %d bytes\n", len(msgBytes))
	fmt.Println()

	// ========================================
	// PART 3: ANY TYPE WRAPPING
	// ========================================

	// Trong Transaction, message được wrap trong Any type
	anyMsg, err := codectypes.NewAnyWithValue(msg)
	if err != nil {
		panic(err)
	}

	fmt.Println("========== ANY TYPE WRAPPING ==========")
	fmt.Printf("TypeURL: %s\n", anyMsg.TypeUrl)
	fmt.Printf("Value (hex): %s\n", hex.EncodeToString(anyMsg.Value))
	fmt.Println()

	// ========================================
	// PART 4: TRANSACTION BODY
	// ========================================

	// Tạo TxBody chứa messages
	txBody := &txtypes.TxBody{
		Messages:      []*codectypes.Any{anyMsg},
		Memo:          "Test transaction",
		TimeoutHeight: 0,
	}

	// Encode TxBody
	txBodyBytes, err := protoCodec.Marshal(txBody)
	if err != nil {
		panic(err)
	}

	fmt.Println("========== TRANSACTION BODY ==========")
	fmt.Printf("TxBody (hex): %s\n", hex.EncodeToString(txBodyBytes))
	fmt.Printf("TxBody (base64): %s\n", base64.StdEncoding.EncodeToString(txBodyBytes))
	fmt.Printf("Size: %d bytes\n", len(txBodyBytes))
	fmt.Println()

	// ========================================
	// PART 5: COMPLETE TRANSACTION
	// ========================================

	pubKey := privKey.PubKey()
	pubKeyAny, err := codectypes.NewAnyWithValue(pubKey)
	if err != nil {
		panic(err)
	}

	sequence := uint64(0)

	// Tạo AuthInfo (chứa fee và signatures info)
	authInfo := &txtypes.AuthInfo{
		SignerInfos: []*txtypes.SignerInfo{
			{
				PublicKey: pubKeyAny,
				ModeInfo: &txtypes.ModeInfo{
					Sum: &txtypes.ModeInfo_Single_{
						Single: &txtypes.ModeInfo_Single{
							Mode: signingtypes.SignMode_SIGN_MODE_DIRECT,
						},
					},
				},
				Sequence: sequence,
			},
		},
		Fee: &txtypes.Fee{
			Amount:   sdk.NewCoins(sdk.NewCoin("uatom", math.NewInt(5000))),
			GasLimit: 200000,
			Payer:    "",
			Granter:  "",
		},
	}

	authInfoBytes, err := protoCodec.Marshal(authInfo)
	if err != nil {
		panic(err)
	}

	chainID := "demo-chain"
	accountNumber := uint64(0)

	signDoc := &txtypes.SignDoc{
		BodyBytes:     txBodyBytes,
		AuthInfoBytes: authInfoBytes,
		ChainId:       chainID,
		AccountNumber: accountNumber,
	}

	signDocBytes, err := gogoproto.Marshal(signDoc)
	if err != nil {
		panic(err)
	}

	signature, err := privKey.Sign(signDocBytes)
	if err != nil {
		panic(err)
	}

	// Complete Transaction
	txRaw := &txtypes.TxRaw{
		BodyBytes:     txBodyBytes,
		AuthInfoBytes: authInfoBytes,
		Signatures:    [][]byte{signature},
	}

	txBytes, err = protoCodec.Marshal(txRaw)
	if err != nil {
		panic(err)
	}

	fmt.Println("========== SIGNATURE ==========")
	fmt.Printf("Chain ID: %s\n", chainID)
	fmt.Printf("AccountNumber: %d | Sequence: %d\n", accountNumber, sequence)
	fmt.Printf("Signature (hex): %s\n", hex.EncodeToString(signature))
	fmt.Printf("Signature (base64): %s\n", base64.StdEncoding.EncodeToString(signature))
	fmt.Println()

	fmt.Println("========== COMPLETE TRANSACTION ==========")
	fmt.Printf("Tx (hex): %s\n", hex.EncodeToString(txBytes))
	fmt.Printf("Tx (base64): %s\n", base64.StdEncoding.EncodeToString(txBytes))
	fmt.Printf("Size: %d bytes\n", len(txBytes))
	fmt.Println()

	return txBytes
}

func Decode(txBytes []byte) {
	// ========================================
	// PART 6: DECODING (Reverse process)
	// ========================================
	// Setup codec (Protobuf encoder/decoder)
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	protoCodec := codec.NewProtoCodec(interfaceRegistry)

	fmt.Println("========== DECODING ==========")

	// Decode TxRaw
	var decodedTxRaw txtypes.TxRaw
	err := protoCodec.Unmarshal(txBytes, &decodedTxRaw)
	if err != nil {
		panic(err)
	}

	// Decode TxBody
	var decodedTxBody txtypes.TxBody
	err = protoCodec.Unmarshal(decodedTxRaw.BodyBytes, &decodedTxBody)
	if err != nil {
		panic(err)
	}

	// Decode Message từ Any
	var decodedMsg types.MsgSend
	err = protoCodec.Unmarshal(decodedTxBody.Messages[0].Value, &decodedMsg)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Decoded Message:\n")
	fmt.Printf("  From: %s\n", decodedMsg.FromAddress)
	fmt.Printf("  To: %s\n", decodedMsg.ToAddress)
	fmt.Printf("  Amount: %s\n", decodedMsg.Amount)
	fmt.Println()

	// ========================================
	// PART 7: JSON REPRESENTATION
	// ========================================

	fmt.Println("========== JSON REPRESENTATION ==========")

	// Convert to JSON (for API responses)
	jsonBytes, err := protoCodec.MarshalJSON(&decodedMsg)
	if err != nil {
		panic(err)
	}

	// Pretty print
	var prettyJSON map[string]interface{}
	json.Unmarshal(jsonBytes, &prettyJSON)
	prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")

	fmt.Printf("Message as JSON:\n%s\n", string(prettyBytes))
	fmt.Println()

	// ========================================
	// PART 8: SIGN BYTES (for signing)
	// ========================================

	fmt.Println("========== SIGN BYTES ==========")
	// fmt.Printf("SignDoc (hex): %s\n", hex.EncodeToString(signDocBytes))
	// fmt.Printf("SignDoc size: %d bytes\n", len(signDocBytes))
	fmt.Println("This is what gets hashed and signed by the private key")
	fmt.Println()

	// ========================================
	// BONUS: Comparison with Amino (Legacy)
	// ========================================

	fmt.Println("========== AMINO ENCODING (LEGACY) ==========")
	aminoCodec := codec.NewLegacyAmino()
	types.RegisterLegacyAminoCodec(aminoCodec)

	aminoBytes, err := aminoCodec.MarshalJSON(&decodedMsg)
	if err != nil {
		panic(err)
	}
	fmt.Println(decodedTxBody.Messages[0].Value)
	fmt.Println(hex.EncodeToString(decodedTxBody.Messages[0].Value))
	fmt.Printf("Amino JSON: %s\n", string(aminoBytes))
	fmt.Printf("Amino size: %d bytes\n", len(aminoBytes))
	fmt.Printf("Protobuf size: %d bytes\n", len(decodedTxBody.Messages[0].Value))
	fmt.Printf("Space saved: %.1f%%\n", (1.0-float64(len(decodedTxBody.Messages[0].Value))/float64(len(aminoBytes)))*100)
}

func main() {
	txBytes := Encode()
	Decode(txBytes)
}
