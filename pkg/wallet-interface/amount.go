package wallet_interface

import (
	"math/big"
	"strings"
)

// Amount represents the base monetary unit of a currency. For Bitcoin
// this would be the satoshi. For USD this would be the cent. A big.Int
// is used to ensure there is enough room for currencies with large base
// units like Ethereum.
type Amount big.Int

// NewAmount creates an Amount from an interface. The interface can be
// either a int, int32, int64, uint32, uint64, string (base 10 or scientific
// notation), or big.Int. Anything else will return a zero Amount.
func NewAmount(i interface{}) Amount {
	switch i.(type) {
	case int:
		return Amount(*big.NewInt(int64(i.(int))))
	case int32:
		return Amount(*big.NewInt(int64(i.(int32))))
	case int64:
		return Amount(*big.NewInt(i.(int64)))
	case uint32:
		return Amount(*big.NewInt(int64(i.(uint32))))
	case uint64:
		a := new(big.Int).SetUint64(i.(uint64))
		return Amount(*a)
	case string:
		s := i.(string)

		// Check for scientific notation.
		if strings.Contains(s, ".") && strings.Contains(s, "e+") {
			f, ok := new(big.Float).SetString(s)
			if !ok {
				return Amount(*big.NewInt(0))
			}
			a, _ := f.Int(nil)
			return Amount(*a)
		}

		a, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return Amount(*big.NewInt(0))
		}
		return Amount(*a)
	case *big.Int:
		a := i.(*big.Int)
		return Amount(*a)
	case big.Int:
		return Amount(i.(big.Int))
	default:
		return Amount(*big.NewInt(0))
	}
}

func (a Amount) String() string {
	x := big.Int(a)
	return x.String()
}

func (a Amount) Int64() int64 {
	x := big.Int(a)
	return x.Int64()
}

func (a Amount) IsInt64() bool {
	x := big.Int(a)
	return x.IsInt64()
}

func (a Amount) Uint64() uint64 {
	x := big.Int(a)
	return x.Uint64()
}

func (a Amount) IsUint64() bool {
	x := big.Int(a)
	return x.IsUint64()
}

func (a Amount) Cmp(b Amount) int {
	x := big.Int(a)
	y := big.Int(b)
	return x.Cmp(&y)
}

func (a Amount) Add(b Amount) Amount {
	x := big.Int(a)
	y := big.Int(b)
	z := new(big.Int).Add(&x, &y)
	return NewAmount(z)
}

func (a Amount) Sub(b Amount) Amount {
	x := big.Int(a)
	y := big.Int(b)
	z := new(big.Int).Sub(&x, &y)
	return NewAmount(z)
}

func (a Amount) Mul(b Amount) Amount {
	x := big.Int(a)
	y := big.Int(b)
	z := new(big.Int).Mul(&x, &y)
	return NewAmount(z)
}

func (a Amount) Div(b Amount) Amount {
	x := big.Int(a)
	y := big.Int(b)
	fx := new(big.Float).SetInt(&x)
	fy := new(big.Float).SetInt(&y)
	fz := new(big.Float).Quo(fx, fy)
	z, _ := fz.Int(nil)
	return NewAmount(z)
}

// MarshalJSON is used to marshal the amount to JSON.
func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(a.String()), nil
}
