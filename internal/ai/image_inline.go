package ai

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
)

const maxInlineImageBytes = 4 << 20 // 4MB per image

// InlineImageURLs downloads product images and converts them to data URLs so
// vision providers (e.g. DashScope) do not need to fetch merchant-hosted URLs.
func InlineImageURLs(ctx context.Context, client *http.Client, gatewayOrigin string, allowLoopbackGateway bool, urls []string) ([]string, error) {
	if len(urls) == 0 {
		return urls, nil
	}
	if client == nil {
		client = http.DefaultClient
	}
	out := make([]string, len(urls))
	for i, raw := range urls {
		if strings.HasPrefix(strings.ToLower(raw), "data:image/") {
			out[i] = raw
			continue
		}
		fetchURL, rewrittenToGateway := rewriteMediaCDNToGateway(raw, gatewayOrigin)
		if err := validateInlineFetchURL(fetchURL, gatewayOrigin, allowLoopbackGateway, rewrittenToGateway); err != nil {
			return nil, fmt.Errorf("image %d: %w", i+1, err)
		}
		dataURL, err := fetchAsDataURL(ctx, client, fetchURL)
		if err != nil {
			return nil, fmt.Errorf("image %d: %w", i+1, err)
		}
		out[i] = dataURL
	}
	return out, nil
}

// RewriteMediaCDNToGateway maps CID-based CDN paths that may 404 right after
// upload to the live gateway media route.
func RewriteMediaCDNToGateway(raw, gatewayOrigin string) string {
	rewritten, _ := rewriteMediaCDNToGateway(raw, gatewayOrigin)
	return rewritten
}

func rewriteMediaCDNToGateway(raw, gatewayOrigin string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw, false
	}
	cidValue := mediaCIDFromURLPath(u.Path)
	if cidValue == "" || gatewayOrigin == "" {
		return raw, false
	}
	rewritten := strings.TrimRight(gatewayOrigin, "/") + "/v1/media/images/" + url.PathEscape(cidValue)
	return rewritten, rewritten != raw
}

func mediaCIDFromURLPath(rawPath string) string {
	segments := strings.Split(strings.Trim(rawPath, "/"), "/")
	for i := len(segments) - 1; i >= 0; i-- {
		segment, err := url.PathUnescape(segments[i])
		if err != nil || segment == "" {
			continue
		}
		if _, err := cid.Decode(segment); err == nil {
			return segment
		}
	}
	return ""
}

func validateInlineFetchURL(rawURL, gatewayOrigin string, allowLoopbackGateway bool, rewrittenToGateway bool) error {
	if err := validateImageURL(rawURL); err == nil {
		return nil
	} else if !rewrittenToGateway || !allowLoopbackGateway || !isLoopbackGatewayMediaURL(rawURL, gatewayOrigin) {
		return err
	}
	return nil
}

func isLoopbackGatewayMediaURL(rawURL, gatewayOrigin string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	origin, err := url.Parse(strings.TrimRight(gatewayOrigin, "/"))
	if err != nil || origin.Host == "" {
		return false
	}
	if !strings.EqualFold(u.Scheme, origin.Scheme) || !strings.EqualFold(u.Host, origin.Host) {
		return false
	}
	if !strings.HasPrefix(u.EscapedPath(), "/v1/media/images/") {
		return false
	}
	return isLoopbackHost(u.Hostname())
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func fetchAsDataURL(ctx context.Context, client *http.Client, imageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	fetchClient := client
	if fetchClient.Timeout == 0 {
		fetchClient = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := fetchClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch returned HTTP %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxInlineImageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty image body")
	}
	if len(data) > maxInlineImageBytes {
		return "", fmt.Errorf("image too large (max %d bytes)", maxInlineImageBytes)
	}

	mime, err := imageMIMEFromResponse(resp.Header.Get("Content-Type"), data)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

func imageMIMEFromResponse(contentType string, data []byte) (string, error) {
	declared := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	detected := detectImageMIME(data)
	if declared == "" || declared == "application/octet-stream" {
		if detected == "" {
			return "", fmt.Errorf("unsupported image content")
		}
		return detected, nil
	}
	if !strings.HasPrefix(declared, "image/") {
		return "", fmt.Errorf("unsupported image content type %q", declared)
	}
	if detected == "" {
		return "", fmt.Errorf("unsupported image content")
	}
	if !sameImageMIME(declared, detected) {
		return "", fmt.Errorf("image content type %q does not match body", declared)
	}
	return detected, nil
}

func sameImageMIME(a, b string) bool {
	if a == "image/jpg" {
		a = "image/jpeg"
	}
	if b == "image/jpg" {
		b = "image/jpeg"
	}
	return a == b
}

func detectImageMIME(data []byte) string {
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 8 && string(data[0:8]) == "\x89PNG\r\n\x1a\n" {
		return "image/png"
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	if len(data) >= 6 && (string(data[0:6]) == "GIF87a" || string(data[0:6]) == "GIF89a") {
		return "image/gif"
	}
	return ""
}
