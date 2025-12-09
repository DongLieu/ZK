package txcodec

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
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

func newProtoCodec() *codec.ProtoCodec {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	types.RegisterInterfaces(interfaceRegistry)
	return codec.NewProtoCodec(interfaceRegistry)
}

func newAminoCodec() *codec.LegacyAmino {
	amino := codec.NewLegacyAmino()
	std.RegisterLegacyAminoCodec(amino)
	types.RegisterLegacyAminoCodec(amino)
	return amino
}

func Encode() (txBytes []byte, msgType string, fromAddr string, amountStr string, denom string) {
	// ========================================
	// PART 1: Tạo key, sinh địa chỉ và message
	// ========================================
	protoCodec := newProtoCodec()

	privKey := secp256k1.GenPrivKey()
	fromAddr = sdk.AccAddress(privKey.PubKey().Address()).String()

	// tạo địa chỉ nhận ngẫu nhiên để minh họa
	receiverPrivKey := secp256k1.GenPrivKey()
	toAddr := sdk.AccAddress(receiverPrivKey.PubKey().Address()).String()

	fmt.Println("========== KEY INFO ==========")
	fmt.Printf("Sender private key (hex): %s\n", hex.EncodeToString(privKey.Bytes()))
	fmt.Printf("Sender address: %s\n", fromAddr)
	fmt.Printf("Receiver address: %s\n", toAddr)
	fmt.Println()

	amountInt := math.NewInt(1000000)
	denom = "uatom"

	// Tạo MsgSend
	msg := &types.MsgSend{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount: sdk.NewCoins(
			sdk.NewCoin(denom, amountInt), // 1 ATOM = 1,000,000 uatom
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

	return txBytes, anyMsg.TypeUrl, fromAddr, amountInt.String(), denom
}

func Decode(txBytes []byte) {
	protoCodec := newProtoCodec()
	aminoCodec := newAminoCodec()

	fmt.Println("========== DECODING ==========")

	var decodedTxRaw txtypes.TxRaw
	if err := protoCodec.Unmarshal(txBytes, &decodedTxRaw); err != nil {
		panic(err)
	}

	var decodedTxBody txtypes.TxBody
	if err := protoCodec.Unmarshal(decodedTxRaw.BodyBytes, &decodedTxBody); err != nil {
		panic(err)
	}

	fmt.Printf("Memo: %s\n", decodedTxBody.Memo)
	fmt.Printf("Messages: %d\n", len(decodedTxBody.Messages))
	fmt.Println()

	for i, anyMsg := range decodedTxBody.Messages {
		fmt.Printf("-- Message #%d --\n", i+1)
		fmt.Printf("TypeURL: %s\n", anyMsg.TypeUrl)
		fmt.Printf("Size: %d bytes\n", len(anyMsg.Value))
		fmt.Printf("Value (hex): %s\n", hex.EncodeToString(anyMsg.Value))
		fmt.Printf("Value (base64): %s\n", base64.StdEncoding.EncodeToString(anyMsg.Value))

		var sdkMsg sdk.Msg
		if err := protoCodec.UnpackAny(anyMsg, &sdkMsg); err != nil {
			fmt.Printf("  ❌ unable to unpack SDK message: %v\n\n", err)
			continue
		}

		jsonBytes, err := protoCodec.MarshalInterfaceJSON(sdkMsg)
		if err != nil {
			fmt.Printf("  ❌ failed to marshal JSON: %v\n\n", err)
			continue
		}

		var prettyJSON map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &prettyJSON); err != nil {
			fmt.Printf("  JSON: %s\n", string(jsonBytes))
		} else {
			prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
			fmt.Printf("  JSON:\n%s\n", string(prettyBytes))
		}

		if aminoBytes, err := aminoCodec.MarshalJSON(sdkMsg); err == nil {
			fmt.Printf("  Amino JSON: %s\n", string(aminoBytes))
			fmt.Printf("  Proto bytes: %d | Amino bytes: %d\n", len(anyMsg.Value), len(aminoBytes))
		} else {
			fmt.Printf("  Amino encoding unavailable: %v\n", err)
		}

		fmt.Println()
	}

	fmt.Println("========== SIGN BYTES ==========")
	fmt.Printf("BodyBytes (hex): %s\n", hex.EncodeToString(decodedTxRaw.BodyBytes))
	fmt.Printf("AuthInfoBytes (hex): %s\n", hex.EncodeToString(decodedTxRaw.AuthInfoBytes))
	for i, sig := range decodedTxRaw.Signatures {
		fmt.Printf("Signature #%d (hex): %s\n", i+1, hex.EncodeToString(sig))
		fmt.Printf("Signature #%d (base64): %s\n", i+1, base64.StdEncoding.EncodeToString(sig))
	}
}
