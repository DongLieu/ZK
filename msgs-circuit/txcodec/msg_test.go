package txcodec

import "testing"

func TestXxx(t *testing.T) {
	txBytes, _, _, _, _ := Encode()
	Decode(txBytes)
}
