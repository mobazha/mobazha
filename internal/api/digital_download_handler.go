package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/response"
)

// handleDigitalDownload serves binary file content for a signed download URL.
// Authentication is provided entirely by the HMAC signature embedded in the
// URL — buyers receive these URLs from the buyer portal API and may consume
// them without an active session (e.g. resuming a download from email).
//
// Path:    GET /v1/orders/{orderID}/digital-download
// Query:   asset, exp, gv, kv, sig (all required)
//
// Validation order matters: the signature is verified first to avoid
// signature oracle attacks against grant lookup.
func (g *Gateway) handleDigitalDownload(w http.ResponseWriter, r *http.Request) {
	// No explicit feature flag gate here — the endpoint is naturally gated by
	// grant existence: grants are only created when digital auto-delivery is
	// enabled, so an attacker probing without a valid grant gets a 403 below.
	// This mirrors the Buyer Portal API which also has no top-level digital
	// flag check.
	svc, ok := getDigitalAssetService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "digital asset subsystem not available")
		return
	}

	// Parse query params produced by GetBuyerDigitalAssets:
	//   /v1/orders/{orderID}/digital-download?grant=NONCE&asset=ID&expires=UNIX&v=GRANT_VERSION&sig=HEX
	orderID := chi.URLParam(r, "orderID")
	q := r.URL.Query()
	nonce := q.Get("grant")
	assetID := q.Get("asset")
	expStr := q.Get("expires")
	vStr := q.Get("v")
	sigHex := q.Get("sig")

	if orderID == "" || assetID == "" || expStr == "" || vStr == "" || sigHex == "" || nonce == "" {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "missing required download parameters")
		return
	}

	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid expires")
		return
	}
	gv, err := strconv.Atoi(vStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid v")
		return
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "invalid sig")
		return
	}

	req := contracts.DownloadRequest{
		OrderID:      orderID,
		GrantNonce:   nonce,
		AssetID:      assetID,
		ExpiryUnix:   exp,
		GrantVersion: gv,
		Signature:    sig,
		BuyerIPHash:  hashClientAddr(r),
		UserAgent:    r.UserAgent(),
	}

	resp, err := svc.ServeDownload(r.Context(), req)
	if err != nil {
		// Fail closed with a generic 403 — leaking the precise reason
		// (signature mismatch vs revoked vs limit reached) helps attackers
		// probe valid grants. Logs / RecordDownload audit captures the
		// detail server-side.
		response.Error(w, http.StatusForbidden, response.CodeForbidden, "download not allowed")
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, sanitizeFileName(resp.FileName)))
	if resp.FileSize > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(resp.FileSize, 10))
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")

	if _, err := io.Copy(w, resp.Body); err != nil {
		// Connection broke mid-stream; nothing more we can do — headers
		// already flushed. Log via the package logger for diagnostics.
		log.Warningf("digital-download: stream interrupted for order %s asset %s: %v", orderID, assetID, err)
	}
}

// hashClientAddr returns a SHA-256 prefix of the client IP for audit logging
// without storing raw addresses (privacy-preserving).
func hashClientAddr(r *http.Request) string {
	addr := r.RemoteAddr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		addr = h
	}
	if addr == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(addr))
	return hex.EncodeToString(sum[:8])
}

// sanitizeFileName strips characters that would break a Content-Disposition
// header. Quote-stripping is sufficient for the simple `filename="..."`
// form; for non-ASCII filenames we'd add filename* / RFC 5987 in a follow-up.
func sanitizeFileName(name string) string {
	if name == "" {
		return "download.bin"
	}
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '"' || r == '\\' || r == '\n' || r == '\r' {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return "download.bin"
	}
	return string(out)
}
