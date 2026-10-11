// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package orders

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	opb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/encoding/protojson"
)

// Buyer requests ("wanted" board entries) are signed public records: the node
// that owns a request publishes it to the hosted index, and the index verifies
// the signature before it shows the record to anyone. Everything the second
// implementation needs — the domain tags, the exact bytes that are signed, and
// the limits — lives here so both sides cannot drift apart.
//
// A signature covers the tag, the owner and the serialized record exactly as it
// was transmitted, each field prefixed with its byte length. The index therefore
// verifies the bytes it received instead of re-encoding the record and hoping
// two encoders agree, and a length prefix is what keeps two adjacent fields from
// being read as one.
const (
	// BuyerRequestSignatureDomain signs a publication.
	BuyerRequestSignatureDomain = "mobazha:buyer-request:v1"
	// BuyerRequestWithdrawDomain signs a withdrawal. A separate tag keeps a
	// publication signature from being replayed as a withdrawal and vice versa.
	BuyerRequestWithdrawDomain = "mobazha:buyer-request-withdraw:v1"

	// BuyerRequestMaxQuantity and the other ceilings mirror what the index
	// enforces, so a buyer is told by their own node instead of by a refused
	// push.
	BuyerRequestMaxQuantity = 1_000_000_000
	// BuyerRequestMaxBudgetDigits bounds a budget amount: it is a count of the
	// currency's smallest unit and may be far larger than a 64-bit integer.
	BuyerRequestMaxBudgetDigits = 40
	// BuyerRequestMaxProductTypeLength bounds a category name.
	BuyerRequestMaxProductTypeLength = 64
	// BuyerRequestMaxCurrencies bounds the accepted-currency list.
	BuyerRequestMaxCurrencies = 32
	// BuyerRequestMaxSerializedBytes is the largest record the index accepts.
	BuyerRequestMaxSerializedBytes = 16 * 1024

	BuyerRequestStatusOpen   = "open"
	BuyerRequestStatusClosed = "closed"

	BuyerRequestMaxTitleLength       = 250
	BuyerRequestMaxDescriptionLength = 1000
	BuyerRequestMaxImages            = 8
	// BuyerRequestMaxOpenPerPeer bounds how much one node may put on the board.
	// The index applies the same ceiling again; it cannot trust a publisher.
	BuyerRequestMaxOpenPerPeer = 10
	// BuyerRequestMaxLifetime is the longest a request may stay open.
	BuyerRequestMaxLifetime = 30 * 24 * time.Hour
	// BuyerRequestDefaultLifetime is what a client uses when the buyer does not
	// say, so "keep it up for a week" needs no input.
	BuyerRequestDefaultLifetime = 7 * 24 * time.Hour
)

// Errors a caller has to tell apart. The API layer maps them onto responses; the
// index service maps them onto its own refusals.
var (
	// ErrBuyerRequestInvalid marks a record that breaks a rule above.
	ErrBuyerRequestInvalid = errors.New("invalid buyer request")
	// ErrBuyerRequestStaleRevision marks a publication that is not newer than
	// what the index already holds.
	ErrBuyerRequestStaleRevision = errors.New("buyer request revision is not newer")
	// ErrBuyerRequestSignature marks a signature that does not verify.
	ErrBuyerRequestSignature = errors.New("buyer request signature is invalid")
)

// MarshalBuyerRequest returns the bytes a publisher transmits and signs. It is
// deliberately the only encoder: a record is signed over these bytes, and the
// index verifies the bytes it received, so neither side re-encodes.
func MarshalBuyerRequest(request *opb.BuyerRequest) ([]byte, error) {
	if request == nil {
		return nil, fmt.Errorf("%w: request is required", ErrBuyerRequestInvalid)
	}
	marshaler := protojson.MarshalOptions{EmitUnpopulated: false, Indent: "    "}
	pretty := marshaler.Format(request)
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(pretty), "", "    "); err != nil {
		return nil, fmt.Errorf("encode buyer request: %w", err)
	}
	return out.Bytes(), nil
}

// ParseBuyerRequest reads a transmitted record back.
func ParseBuyerRequest(serialized []byte) (*opb.BuyerRequest, error) {
	request := new(opb.BuyerRequest)
	unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshaler.Unmarshal(serialized, request); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBuyerRequestInvalid, err)
	}
	return request, nil
}

// BuyerRequestSignaturePayload returns the bytes covered by a publication
// signature:
//
//	mobazha:buyer-request:v1 \n {len}:{peerID} \n {len}:{serialized record}
//
// Every value is prefixed with its length in bytes, so no two neighbouring
// values can be re-split into a different record. There is no trailing newline:
// the record's own bytes end the payload, which is what the hosted index
// verifies.
func BuyerRequestSignaturePayload(request *opb.BuyerRequest, serialized []byte) ([]byte, error) {
	if request == nil {
		return nil, fmt.Errorf("%w: request is required", ErrBuyerRequestInvalid)
	}
	if len(serialized) == 0 {
		return nil, fmt.Errorf("%w: serialized request is required", ErrBuyerRequestInvalid)
	}
	peerID := strings.TrimSpace(request.GetPeerID())
	if peerID == "" {
		return nil, fmt.Errorf("%w: peerID is required", ErrBuyerRequestInvalid)
	}

	payload := make([]byte, 0, len(serialized)+len(peerID)+48)
	payload = append(payload, BuyerRequestSignatureDomain...)
	payload = appendLengthPrefixed(payload, peerID)
	payload = appendLengthPrefixed(payload, string(serialized))
	return payload, nil
}

// BuyerRequestWithdrawPayload returns the bytes covered by a withdrawal
// signature:
//
//	mobazha:buyer-request-withdraw:v1 \n {len}:{peerID} \n {len}:{requestID}
//	  \n {len}:{revision} \n
//
// The identifier and the revision are all the index needs to know that the
// owner wants this record off the board, and a trailing newline closes the
// payload.
func BuyerRequestWithdrawPayload(peerID, requestID string, revision uint32) []byte {
	payload := make([]byte, 0, 128)
	payload = append(payload, BuyerRequestWithdrawDomain...)
	payload = appendLengthPrefixed(payload, strings.TrimSpace(peerID))
	payload = appendLengthPrefixed(payload, strings.TrimSpace(requestID))
	payload = appendLengthPrefixed(payload, strconv.FormatUint(uint64(revision), 10))
	return append(payload, '\n')
}

// appendLengthPrefixed writes "\n<byte length>:<value>", which is how every
// field in a signature payload is framed.
func appendLengthPrefixed(payload []byte, value string) []byte {
	payload = append(payload, '\n')
	payload = strconv.AppendInt(payload, int64(len(value)), 10)
	payload = append(payload, ':')
	return append(payload, value...)
}

// SignBuyerRequest validates nothing; callers validate first so a buyer sees the
// problem instead of publishing a record the index will refuse.
func SignBuyerRequest(key crypto.PrivKey, request *opb.BuyerRequest) (serialized, signature []byte, err error) {
	if key == nil {
		return nil, nil, fmt.Errorf("%w: signing key is required", ErrBuyerRequestInvalid)
	}
	serialized, err = MarshalBuyerRequest(request)
	if err != nil {
		return nil, nil, err
	}
	if len(serialized) > BuyerRequestMaxSerializedBytes {
		return nil, nil, fmt.Errorf("%w: the record is larger than %d bytes",
			ErrBuyerRequestInvalid, BuyerRequestMaxSerializedBytes)
	}
	payload, err := BuyerRequestSignaturePayload(request, serialized)
	if err != nil {
		return nil, nil, err
	}
	signature, err = key.Sign(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("sign buyer request: %w", err)
	}
	return serialized, signature, nil
}

// VerifyBuyerRequest checks a publication against the peer it claims to come
// from. A libp2p peer ID carries its own public key, so no lookup is involved
// and a record can only ever be published by the node it names.
func VerifyBuyerRequest(peerID string, serialized, signature []byte) (*opb.BuyerRequest, error) {
	if len(serialized) > BuyerRequestMaxSerializedBytes {
		return nil, fmt.Errorf("%w: the record is larger than %d bytes",
			ErrBuyerRequestInvalid, BuyerRequestMaxSerializedBytes)
	}
	request, err := ParseBuyerRequest(serialized)
	if err != nil {
		return nil, err
	}
	if claimed := strings.TrimSpace(peerID); claimed != "" && claimed != strings.TrimSpace(request.GetPeerID()) {
		return nil, fmt.Errorf("%w: record belongs to %s", ErrBuyerRequestInvalid, request.GetPeerID())
	}
	payload, err := BuyerRequestSignaturePayload(request, serialized)
	if err != nil {
		return nil, err
	}
	if err := VerifyBuyerRequestSignature(request.GetPeerID(), payload, signature); err != nil {
		return nil, err
	}
	return request, nil
}

// VerifyBuyerRequestSignature checks a signature against a peer ID.
func VerifyBuyerRequestSignature(peerID string, payload, signature []byte) error {
	owner, err := peer.Decode(strings.TrimSpace(peerID))
	if err != nil {
		return fmt.Errorf("%w: %q is not a peer ID", ErrBuyerRequestInvalid, peerID)
	}
	pubkey, err := owner.ExtractPublicKey()
	if err != nil {
		return fmt.Errorf("%w: cannot read the publisher key: %s", ErrBuyerRequestSignature, err)
	}
	ok, err := pubkey.Verify(payload, signature)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBuyerRequestSignature, err)
	}
	if !ok {
		return fmt.Errorf("%w: signature does not match %s", ErrBuyerRequestSignature, peerID)
	}
	return nil
}

// ValidateBuyerRequest reports the first rule a record breaks.
//
// These are the index's rules, spelled out once and applied on both sides: the
// node applies them before signing, so a buyer sees the problem in their own
// language instead of as a refused publication, and the index applies them again
// because it cannot trust a publisher.
func ValidateBuyerRequest(request *opb.BuyerRequest, now time.Time) error {
	if request == nil {
		return fmt.Errorf("%w: request is required", ErrBuyerRequestInvalid)
	}
	if err := checkNoControlBytes(request); err != nil {
		return err
	}

	requestID := strings.TrimSpace(request.GetRequestID())
	if requestID == "" {
		return fmt.Errorf("%w: requestID is required", ErrBuyerRequestInvalid)
	}
	if len(requestID) > 128 || !isIdentifier(requestID) {
		return fmt.Errorf("%w: requestID must start with a letter or a digit, "+
			"may contain letters, digits, dashes and underscores, and is at most 128 characters",
			ErrBuyerRequestInvalid)
	}
	if request.GetRevision() < 1 {
		return fmt.Errorf("%w: revision must be at least 1", ErrBuyerRequestInvalid)
	}

	title := strings.TrimSpace(request.GetTitle())
	if title == "" {
		return fmt.Errorf("%w: title is required", ErrBuyerRequestInvalid)
	}
	if utf8.RuneCountInString(title) > BuyerRequestMaxTitleLength {
		return fmt.Errorf("%w: title must be at most %d characters",
			ErrBuyerRequestInvalid, BuyerRequestMaxTitleLength)
	}
	if utf8.RuneCountInString(request.GetDescription()) > BuyerRequestMaxDescriptionLength {
		return fmt.Errorf("%w: description must be at most %d characters",
			ErrBuyerRequestInvalid, BuyerRequestMaxDescriptionLength)
	}
	if request.GetQuantity() < 1 || request.GetQuantity() > BuyerRequestMaxQuantity {
		return fmt.Errorf("%w: quantity must be between 1 and %d",
			ErrBuyerRequestInvalid, BuyerRequestMaxQuantity)
	}

	budget := request.GetBudget()
	if budget == nil || budget.GetCurrency() == nil {
		return fmt.Errorf("%w: budget is required", ErrBuyerRequestInvalid)
	}
	if !isCurrencyCode(budget.GetCurrency().GetCode()) {
		return fmt.Errorf("%w: budget currency must be 2 to 16 characters, "+
			"starting with a letter", ErrBuyerRequestInvalid)
	}
	if budget.GetCurrency().GetDivisibility() > 18 {
		return fmt.Errorf("%w: budget currency divisibility must be at most 18", ErrBuyerRequestInvalid)
	}
	amount := strings.TrimSpace(budget.GetAmount())
	if !isDigits(amount) || len(amount) > BuyerRequestMaxBudgetDigits || isAllZero(amount) {
		return fmt.Errorf("%w: budget amount must be a positive integer in the "+
			"currency's smallest unit, at most %d digits",
			ErrBuyerRequestInvalid, BuyerRequestMaxBudgetDigits)
	}

	currencies := request.GetAcceptedCurrencies()
	if len(currencies) == 0 || len(currencies) > BuyerRequestMaxCurrencies {
		return fmt.Errorf("%w: accepted currencies must number between 1 and %d",
			ErrBuyerRequestInvalid, BuyerRequestMaxCurrencies)
	}
	seen := make(map[string]bool, len(currencies))
	for _, code := range currencies {
		if !isCurrencyCode(code) {
			return fmt.Errorf("%w: %q is not a currency code", ErrBuyerRequestInvalid, code)
		}
		if seen[code] {
			return fmt.Errorf("%w: %q is listed twice", ErrBuyerRequestInvalid, code)
		}
		seen[code] = true
	}

	// shipTo and productType are optional: a buyer who does not care where a
	// seller ships from, or which category their request falls under, is not
	// asked to make something up.
	if shipTo := strings.TrimSpace(request.GetShipTo()); shipTo != "" && !isCountryCode(shipTo) {
		return fmt.Errorf("%w: shipTo must be a two-letter country code", ErrBuyerRequestInvalid)
	}
	if utf8.RuneCountInString(request.GetProductType()) > BuyerRequestMaxProductTypeLength {
		return fmt.Errorf("%w: productType must be at most %d characters",
			ErrBuyerRequestInvalid, BuyerRequestMaxProductTypeLength)
	}

	if len(request.GetImages()) > BuyerRequestMaxImages {
		return fmt.Errorf("%w: at most %d images are allowed", ErrBuyerRequestInvalid, BuyerRequestMaxImages)
	}
	for _, image := range request.GetImages() {
		if err := checkBuyerRequestImage(image); err != nil {
			return err
		}
	}

	status := request.GetStatus()
	if status != BuyerRequestStatusOpen && status != BuyerRequestStatusClosed {
		return fmt.Errorf("%w: status must be open or closed", ErrBuyerRequestInvalid)
	}

	createdAt := request.GetCreatedAt().AsTime()
	expiresAt := request.GetExpiresAt().AsTime()
	if createdAt.IsZero() || expiresAt.IsZero() {
		return fmt.Errorf("%w: createdAt and expiresAt are required", ErrBuyerRequestInvalid)
	}
	if !expiresAt.After(createdAt) {
		return fmt.Errorf("%w: expiresAt must be after createdAt", ErrBuyerRequestInvalid)
	}
	if expiresAt.Sub(createdAt) > BuyerRequestMaxLifetime {
		return fmt.Errorf("%w: a request may stay open for at most %d days",
			ErrBuyerRequestInvalid, int(BuyerRequestMaxLifetime.Hours()/24))
	}
	if now.Unix() > 0 {
		// A record may be a few minutes ahead of the verifier: clocks drift, and
		// refusing a fresh record over it would be indistinguishable from a bug
		// in the signer.
		if createdAt.After(now.Add(5 * time.Minute)) {
			return fmt.Errorf("%w: createdAt is in the future", ErrBuyerRequestInvalid)
		}
		if status == BuyerRequestStatusOpen && !expiresAt.After(now) {
			return fmt.Errorf("%w: expiresAt must be in the future", ErrBuyerRequestInvalid)
		}
	}
	return nil
}

// checkNoControlBytes refuses a record carrying a NUL byte: a field with one can
// hide a second value behind the check that reads the first.
func checkNoControlBytes(request *opb.BuyerRequest) error {
	fields := []string{
		request.GetRequestID(), request.GetPeerID(), request.GetTitle(),
		request.GetDescription(), request.GetShipTo(), request.GetProductType(),
		request.GetBudget().GetCurrency().GetCode(), request.GetBudget().GetAmount(),
	}
	fields = append(fields, request.GetAcceptedCurrencies()...)
	for _, field := range fields {
		if strings.ContainsRune(field, 0) {
			return fmt.Errorf("%w: no field may contain a NUL character", ErrBuyerRequestInvalid)
		}
	}
	return nil
}

// checkBuyerRequestImage applies the one rule an image entry has to satisfy: it
// must carry a hash to render.
func checkBuyerRequestImage(image *opb.Image) error {
	if image == nil {
		return fmt.Errorf("%w: an image entry is empty", ErrBuyerRequestInvalid)
	}
	if strings.TrimSpace(image.GetOriginal()) == "" && strings.TrimSpace(image.GetTiny()) == "" &&
		strings.TrimSpace(image.GetSmall()) == "" && strings.TrimSpace(image.GetMedium()) == "" &&
		strings.TrimSpace(image.GetLarge()) == "" {
		return fmt.Errorf("%w: an image entry needs at least one hash", ErrBuyerRequestInvalid)
	}
	if utf8.RuneCountInString(image.GetFilename()) > 255 || utf8.RuneCountInString(image.GetAlt()) > 255 {
		return fmt.Errorf("%w: an image filename or alt text must be at most 255 characters",
			ErrBuyerRequestInvalid)
	}
	return nil
}

// isIdentifier accepts what an identifier may start with and contain.
func isIdentifier(value string) bool {
	for index, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case (r == '-' || r == '_') && index > 0:
		default:
			return false
		}
	}
	return true
}

// isCurrencyCode accepts a currency code the way the board names currencies:
// two to sixteen characters, starting with a letter and otherwise upper-case
// letters or digits.
func isCurrencyCode(code string) bool {
	if len(code) < 2 || len(code) > 16 {
		return false
	}
	for index, r := range code {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9' && index > 0:
		default:
			return false
		}
	}
	return true
}

// isCountryCode accepts the two-letter country a request ships to.
func isCountryCode(code string) bool {
	if len(code) != 2 {
		return false
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// isDigits reports whether a string is a run of decimal digits.
func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isAllZero reports whether a run of digits is zero, which is not an amount.
func isAllZero(value string) bool {
	for _, r := range value {
		if r != '0' {
			return false
		}
	}
	return true
}

// IsBuyerRequestListed reports whether a record belongs on the board at a given
// moment. A reader must treat a record past its deadline as closed even when the
// owning node never sent a withdrawal, because a node only sweeps expiry when
// something reads its own board.
func IsBuyerRequestListed(request *opb.BuyerRequest, now time.Time) bool {
	if request == nil {
		return false
	}
	return request.GetStatus() == BuyerRequestStatusOpen && request.GetExpiresAt().AsTime().After(now)
}
