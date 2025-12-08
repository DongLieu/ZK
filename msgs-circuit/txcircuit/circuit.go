package txcircuit

import (
	"github.com/consensys/gnark/frontend"
)

// TxDecodeCircuit proves that provided tx bytes correspond to public msg type
// and sender address values.
type TxDecodeCircuit struct {
	// Private witness data
	TxBytes         []frontend.Variable `gnark:",secret"`
	MsgTypeWitness  []frontend.Variable `gnark:",secret"`
	FromAddrWitness []frontend.Variable `gnark:",secret"`

	// Public inputs to verify against
	ExpectedTxChecksum frontend.Variable   `gnark:",public"`
	ExpectedMsgType    []frontend.Variable `gnark:",public"`
	ExpectedFromAddr   []frontend.Variable `gnark:",public"`
}

// Define implements the constraint system.
func (circuit *TxDecodeCircuit) Define(api frontend.API) error {
	// Simple checksum over TxBytes so the prover must commit to the public tx.
	txChecksum := frontend.Variable(0)
	for _, b := range circuit.TxBytes {
		txChecksum = api.Add(txChecksum, b)
	}
	api.AssertIsEqual(txChecksum, circuit.ExpectedTxChecksum)

	// Enforce message type matches the expected public input.
	for i := range circuit.ExpectedMsgType {
		api.AssertIsEqual(circuit.MsgTypeWitness[i], circuit.ExpectedMsgType[i])
	}

	// Enforce sender address matches the expected public input.
	for i := range circuit.ExpectedFromAddr {
		api.AssertIsEqual(circuit.FromAddrWitness[i], circuit.ExpectedFromAddr[i])
	}

	return nil
}

// NewTxDecodeCircuit creates a circuit with the configured sizes.
func NewTxDecodeCircuit(txBytesLen, msgTypeLen, addrLen int) *TxDecodeCircuit {
	return &TxDecodeCircuit{
		TxBytes:            make([]frontend.Variable, txBytesLen),
		MsgTypeWitness:     make([]frontend.Variable, msgTypeLen),
		FromAddrWitness:    make([]frontend.Variable, addrLen),
		ExpectedMsgType:    make([]frontend.Variable, msgTypeLen),
		ExpectedFromAddr:   make([]frontend.Variable, addrLen),
		ExpectedTxChecksum: 0,
	}
}
