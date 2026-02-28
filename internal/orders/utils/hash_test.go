package utils_test

import (
	"encoding/hex"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
)

func TestMultihashSha256(t *testing.T) {
	h := "c560021782de34f597ef1b2bd415d20c7febe7f111e6c1da349990323e082c74"
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatal(err)
	}
	mh, err := utils.MultihashSha256(b)
	if err != nil {
		t.Fatal(err)
	}

	expected := "QmRPibjjnNE4FrvbKzADHr98PEopP6Mv1Zzw48C2atEt6q"
	if mh.B58String() != expected {
		t.Errorf("Incorrect hash returned expected %s, got %s", expected, mh.B58String())
	}
}

func TestHashListing(t *testing.T) {
	sl := factory.NewSignedListing()

	mh, err := utils.HashListing(sl)
	if err != nil {
		t.Fatal(err)
	}

	expected := "QmX1SpvqrMzLejgEhnvRQ85keTvkhYg5qnAG3NtTD47KTw"
	if mh.B58String() != expected {
		t.Errorf("Incorrect hash returned expected %s, got %s", expected, mh.B58String())
	}
}
