package evm

import "errors"

var (
	// ErrFactoryNotSet is returned when DefaultFactory has not been initialized.
	// This typically means the internal EVM package has not been imported.
	ErrFactoryNotSet = errors.New("evm: DefaultFactory not set (import internal/chains/evm to register)")
)
