// Package guest implements the Guest Checkout domain: anonymous direct-payment
// orders without escrow, HD address derivation, auto-sweep, and payment monitoring.
package guest

import "github.com/mobazha/mobazha/pkg/logging"

var log = logging.MustGetLogger("GUEST")
