package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/response"
)

func classifyGuestSupplyQuoteError(err error) (status int, code string, message string) {
	if err == nil {
		return http.StatusOK, "", ""
	}
	if errors.Is(err, contracts.ErrGuestCheckoutDisabled) {
		return http.StatusForbidden, response.CodeForbidden, "Guest Checkout is not available"
	}
	return classifyCheckoutSupplyQuoteError(err)
}

func classifyCheckoutSupplyQuoteError(err error) (status int, code string, message string) {
	status, code = classifyCheckoutSupplyQuoteStatus(err)
	message = supplyQuoteClientMessage(err, status, code)
	return status, code, message
}

func classifyCheckoutSupplyQuoteStatus(err error) (status int, code string) {
	if err == nil {
		return http.StatusOK, ""
	}
	if errors.Is(err, contracts.ErrInvalidGuestRequest) || errors.Is(err, contracts.ErrInvalidVariant) {
		return http.StatusBadRequest, response.CodeBadRequest
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not configured") {
		return http.StatusNotImplemented, response.CodeNotImplemented
	}
	switch {
	case strings.Contains(msg, "not found"),
		strings.Contains(msg, "must be positive"),
		strings.Contains(msg, "mixed pricing"),
		strings.Contains(msg, "at least one item"),
		strings.Contains(msg, "do not match any sku"),
		strings.Contains(msg, "no pricing currency"):
		return http.StatusBadRequest, response.CodeBadRequest
	default:
		return http.StatusInternalServerError, response.CodeInternalError
	}
}

func supplyQuoteClientMessage(err error, status int, code string) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, contracts.ErrGuestCheckoutDisabled):
		return "Guest Checkout is not available"
	case code == response.CodeNotImplemented:
		return "Supply quote is not available"
	case status == http.StatusInternalServerError:
		return "Unable to check availability"
	case status == http.StatusServiceUnavailable:
		return "Checkout is temporarily unavailable"
	case status == http.StatusConflict:
		if errors.Is(err, contracts.ErrInsufficientStock) {
			return "Insufficient stock"
		}
		return "Unable to complete checkout for these items"
	case errors.Is(err, contracts.ErrInvalidGuestRequest):
		return "Invalid checkout request"
	case errors.Is(err, contracts.ErrInvalidVariant):
		return "Selected variant is invalid"
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not found"):
		return "One or more items could not be found"
	case strings.Contains(msg, "must be positive"):
		return "Item quantity must be positive"
	case strings.Contains(msg, "mixed pricing"):
		return "Items must share the same pricing currency"
	case strings.Contains(msg, "at least one item"):
		return "At least one item is required"
	default:
		return "Invalid checkout request"
	}
}
