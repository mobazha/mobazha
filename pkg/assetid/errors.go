package assetid

import (
	"errors"
	"fmt"
)

// ErrorCode identifies a validation or registry failure reason.
type ErrorCode string

const (
	ErrCodeEmpty               ErrorCode = "ASSET_ID_EMPTY"
	ErrCodeInvalidSegmentCount ErrorCode = "ASSET_ID_INVALID_SEGMENT_COUNT"
	ErrCodeInvalidPrefix       ErrorCode = "ASSET_ID_INVALID_PREFIX"
	ErrCodeInvalidNamespace    ErrorCode = "ASSET_ID_INVALID_NAMESPACE"
	ErrCodeInvalidChainRef     ErrorCode = "ASSET_ID_INVALID_CHAIN_REF"
	ErrCodeInvalidNativeFormat ErrorCode = "ASSET_ID_INVALID_NATIVE_FORMAT"
	ErrCodeInvalidStandard     ErrorCode = "ASSET_ID_INVALID_STANDARD"
	ErrCodeInvalidAssetRef     ErrorCode = "ASSET_ID_INVALID_ASSET_REF"
	ErrCodeInvalidEVMAddress   ErrorCode = "ASSET_ID_INVALID_EIP155_ADDRESS"
	ErrCodeInvalidTRONAddress  ErrorCode = "ASSET_ID_INVALID_TRON_ADDRESS"
	ErrCodeInvalidSolanaMint   ErrorCode = "ASSET_ID_INVALID_SOLANA_MINT"
	ErrCodeUnknownAsset        ErrorCode = "ASSET_ID_UNKNOWN"
)

// Error carries a stable machine-readable code plus an optional human detail.
type Error struct {
	Code   ErrorCode
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

func newError(code ErrorCode, detail string) error {
	return &Error{Code: code, Detail: detail}
}

// IsCode reports whether err is an assetid Error with the given code.
func IsCode(err error, code ErrorCode) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}
