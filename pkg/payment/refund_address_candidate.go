package payment

import "strings"

type refundAddressSameFunc func(a, b string) bool

type refundAddressCandidate struct {
	candidate string
	sawRecord bool
}

func (c *refundAddressCandidate) add(raw string, same refundAddressSameFunc) (cont bool, reason string) {
	c.sawRecord = true
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true, ""
	}
	if c.candidate == "" {
		c.candidate = raw
		return true, ""
	}
	if !same(c.candidate, raw) {
		return false, RefundResolveReasonMultiInput
	}
	return true, ""
}

func (c *refundAddressCandidate) result() (addr string, ok bool, reason string) {
	if c.candidate != "" {
		return c.candidate, true, ""
	}
	if c.sawRecord {
		return "", false, RefundResolveReasonUnparseable
	}
	return "", false, RefundResolveReasonNotObservedYet
}

func equalFoldRefundAddress(a, b string) bool {
	return strings.EqualFold(a, b)
}

func sameUTXORefundAddress(a, b string) bool {
	return SameUTXOAddress(a, b)
}
