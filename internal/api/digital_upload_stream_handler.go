package api

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// digitalAssetMaxStreamSize bounds the per-upload size at the gateway. The
// underlying chunked AEAD container can encode arbitrary file sizes up to
// `MaxStreamChunkSize × 2^32` chunks, but in practice we cap at 1 GiB to
// keep buyer download UX (browser memory, mobile network) reasonable. Bump
// when you've verified the storage adapter, audit log volume, and retry
// semantics scale with larger uploads.
const digitalAssetMaxStreamSize int64 = 1 << 30 // 1 GiB

// handlePOSTDigitalAssetUploadStream accepts a multipart/form-data upload
// and streams the file part directly through the v1 chunked AEAD encryptor
// into BlobStore — never buffering the full plaintext or ciphertext in
// process memory.
//
// Form fields (in order):
//
//	listingSlug (text, required, must precede file)
//	variantSku  (text, optional)
//	fileName    (text, optional)
//	mimeType    (text, optional — falls back to the file part's Content-Type)
//	file        (binary, required, MUST be last)
//
// Field order is enforced server-side because the streaming reader cannot
// be repositioned. Once the `file` part is consumed and uploaded the
// handler returns immediately; any trailing parts after `file` are simply
// never read (Go's http server closes the request body after the handler
// returns). Clients should therefore put `file` last in the form.
//
// Auth: piggybacks on the Gateway's AuthenticationMiddleware via the chi
// router (the route is registered behind the same security guard as other
// seller endpoints).
func (g *Gateway) handlePOSTDigitalAssetUploadStream(w http.ResponseWriter, r *http.Request) {
	svc, ok := getDigitalAssetService(r)
	if !ok {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "digital asset subsystem not available")
		return
	}

	// MultipartReader requires Content-Type: multipart/form-data with boundary.
	mr, err := r.MultipartReader()
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "multipart parse: "+err.Error())
		return
	}

	var (
		listingSlug string
		variantSKU  string
		formMime    string
		fileSeen    bool
	)

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "multipart read: "+err.Error())
			return
		}

		formName := part.FormName()
		if formName != "file" {
			if fileSeen {
				_ = part.Close()
				response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
					"unexpected form field after file part: "+formName)
				return
			}
			// Read small text fields fully; cap to a few KB each to prevent
			// abuse via gigabyte-sized "metadata" parts.
			value, rerr := readFormText(part, 4*1024)
			_ = part.Close()
			if rerr != nil {
				response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
					"form field "+formName+": "+rerr.Error())
				return
			}
			switch formName {
			case "listingSlug":
				listingSlug = value
			case "variantSku":
				variantSKU = value
			case "mimeType":
				formMime = value
			}
			continue
		}

		if fileSeen {
			_ = part.Close()
			response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "duplicate file part")
			return
		}
		fileSeen = true

		if listingSlug == "" {
			_ = part.Close()
			response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
				"listingSlug is required and must appear before the file part in the multipart body")
			return
		}

		fileName := managed_escrowFormFileName(part.FileName())
		mimeType := formMime
		if mimeType == "" {
			if ct := part.Header.Get("Content-Type"); ct != "" {
				mt, _, mimeErr := mime.ParseMediaType(ct)
				if mimeErr == nil {
					mimeType = mt
				}
			}
		}

		// Bound the streamed upload at the gateway. http.MaxBytesReader
		// returns ErrorBadRequest with a generic message on overflow; we
		// rewrap to a clear API error.
		bounded := http.MaxBytesReader(w, &maxBytesPartReader{part: part}, digitalAssetMaxStreamSize)
		info, uploadErr := svc.UploadFileAssetStream(r.Context(), listingSlug, variantSKU, fileName, mimeType, bounded, -1)
		_ = part.Close()
		if uploadErr != nil {
			if errors.Is(uploadErr, contracts.ErrDigitalVariantUnsupported) {
				response.Error(w, http.StatusBadRequest, response.CodeBadRequest,
					"variant-specific digital assets are not supported in Phase 1")
				return
			}
			if isMaxBytesError(uploadErr) {
				response.Error(w, http.StatusRequestEntityTooLarge, response.CodeBadRequest,
					"file exceeds maximum upload size of 1 GiB")
				return
			}
			log.Errorf("digital asset stream upload failed: %v", uploadErr)
			response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "failed to upload file asset")
			return
		}

		response.Created(w, info)
		return
	}

	response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "missing file part")
}

// readFormText fully reads a small text form field, rejecting oversize input
// to prevent abuse via huge non-file form values.
func readFormText(r io.Reader, max int64) (string, error) {
	limited := io.LimitReader(r, max+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if int64(len(buf)) > max {
		return "", errors.New("form field too large")
	}
	return strings.TrimSpace(string(buf)), nil
}

// managed_escrowFormFileName mirrors the Content-Disposition sanitization of the
// download path so the stored filename round-trips cleanly.
func managed_escrowFormFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "upload.bin"
	}
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '/' || r == '\\' || r == 0 {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return "upload.bin"
	}
	return string(out)
}

// maxBytesPartReader adapts multipart.Part to io.ReadCloser, which is what
// http.MaxBytesReader expects. The Close is a no-op because the parent loop
// in handlePOSTDigitalAssetUploadStream is already responsible for closing
// the underlying multipart.Part after the upload completes (success or
// error). We don't want MaxBytesReader's wrapper to double-close it.
type maxBytesPartReader struct {
	part io.Reader
}

func (m *maxBytesPartReader) Read(p []byte) (int, error) {
	return m.part.Read(p)
}

func (m *maxBytesPartReader) Close() error { return nil }

// registerDigitalAssetStreamRoute mounts the raw multipart streaming upload
// endpoint on the V1 chi router. It is intentionally NOT a huma operation:
// huma's body-binding pipeline buffers the entire request body, defeating
// the point of streaming. We piggyback on the gateway's
// AuthenticationMiddleware (Bearer JWT / mbz_ token / Basic Auth) so the
// auth model matches the rest of /v1/digital-assets/*.
//
// In SaaS / SharedRouter mode the upstream resolver has already populated
// AuthIdentity in context; AuthenticationMiddleware short-circuits in that
// case (see auth.go). So a single registration covers both deployments.
//
// The body-size middleware (maxBodySizeMiddleware) automatically exempts
// multipart/form-data requests, so the per-route MaxBytesReader inside the
// handler is the only size cap that applies.
func (g *Gateway) registerDigitalAssetStreamRoute(r chi.Router) {
	r.Method(http.MethodPost, "/v1/digital-assets/upload-stream",
		g.AuthenticationMiddleware(http.HandlerFunc(g.handlePOSTDigitalAssetUploadStream)),
	)
}
