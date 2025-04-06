package wallet_interface

// Address represents a cryptocurrency address used by Mobazha.
type Address struct {
	addr string
	typ  CoinType
}

// NewAddress return a new Address.
func NewAddress(addr string, typ CoinType) Address {
	return Address{addr, typ}
}

// String returns the address's string representation.
func (a Address) String() string {
	return a.addr
}

// CoinType returns the addresses type.
func (a Address) CoinType() CoinType {
	return a.typ
}

type AddressEx struct {
	addr   Address
	script []byte
}

// NewAddress return a new AddressEx.
func NewAddressEx(addr Address, script []byte) AddressEx {
	return AddressEx{addr, script}
}

// NewNativeAddressEx return a new AddressEx with native address on chain instead of script hash.
func NewNativeAddressEx(addr Address) AddressEx {
	return AddressEx{addr, []byte{}}
}

// NewNativeAddressesEx return a new AddressEx slice with native addresses on chain instead of script hashes.
func NewNativeAddressesEx(addrs []Address) []AddressEx {
	addrsEx := []AddressEx{}
	for _, addr := range addrs {
		addrsEx = append(addrsEx, NewNativeAddressEx(addr))
	}
	return addrsEx
}

// String returns the wallet or script address
func (a AddressEx) Address() Address {
	return a.addr
}

// String returns the address's string representation.
func (a AddressEx) String() string {
	return a.addr.String()
}

// CoinType returns the addresses type.
func (a AddressEx) CoinType() CoinType {
	return a.addr.CoinType()
}

// Script returns the bound script.
func (a AddressEx) Script() []byte {
	return a.script
}
