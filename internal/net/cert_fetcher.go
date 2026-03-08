package net

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const certFetchTimeout = 10 * time.Second

// FetchCasdoorCertificate retrieves the JWT verification certificate from
// the SaaS platform. This allows standalone nodes to validate JWTs without
// manually configuring the certificate.
//
// Endpoint: GET {saasURL}/platform/v1/auth/certificate
// Response: {"data": {"certificate": "-----BEGIN CERTIFICATE-----\n..."}}
func FetchCasdoorCertificate(ctx context.Context, saasURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, certFetchTimeout)
	defer cancel()

	url := saasURL + "/platform/v1/auth/certificate"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch certificate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Data struct {
			Certificate string `json:"certificate"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if result.Data.Certificate == "" {
		return "", fmt.Errorf("empty certificate in response")
	}

	return result.Data.Certificate, nil
}
