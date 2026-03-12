package orders

import (
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestAddressSet_NonEVM(t *testing.T) {
	s := newAddressSet(iwallet.CtMock.String())

	s.Add("addr1")
	s.Add("addr2")
	s.Add("")

	if !s.Contains("addr1") {
		t.Error("should contain addr1")
	}
	if !s.Contains("addr2") {
		t.Error("should contain addr2")
	}
	if s.Contains("addr3") {
		t.Error("should not contain addr3")
	}
	if s.Contains("") {
		t.Error("should not contain empty string")
	}
	if s.Contains("ADDR1") {
		t.Error("non-EVM should be case-sensitive")
	}
}

func TestAddressSet_EVM_CaseInsensitive(t *testing.T) {
	s := newAddressSet(iwallet.CtBNB.String())

	s.Add("0xAbCdEf1234567890AbCdEf1234567890AbCdEf12")
	s.Add(" 0x111 ")

	if !s.Contains("0xabcdef1234567890abcdef1234567890abcdef12") {
		t.Error("EVM should match lowercase")
	}
	if !s.Contains("0xABCDEF1234567890ABCDEF1234567890ABCDEF12") {
		t.Error("EVM should match uppercase")
	}
	if !s.Contains("0x111") {
		t.Error("should contain trimmed address")
	}
	if s.Contains("0x999") {
		t.Error("should not contain unknown address")
	}
}

func TestAddressSet_UnknownCoin(t *testing.T) {
	s := newAddressSet("UNKNOWN_COIN_XYZ")

	s.Add("addr1")
	if !s.Contains("addr1") {
		t.Error("should contain addr1")
	}
	if s.Contains("ADDR1") {
		t.Error("unknown coin should be case-sensitive")
	}
}

func TestIsZeroAmount(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"0", true},
		{" 0 ", true},
		{" ", true},
		{"1", false},
		{"100", false},
		{"0.0", false},
	}
	for _, tt := range tests {
		if got := isZeroAmount(tt.input); got != tt.want {
			t.Errorf("isZeroAmount(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
