package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mobazha/mobazha3.0/pkg/encryption"
)

const matrixProvisionFile = "matrix_provision.json"

// matrixProvisionState is persisted to disk after a successful SaaS provision,
// so subsequent node restarts can skip the provision call and login directly.
type matrixProvisionState struct {
	HomeserverURL string `json:"homeserver_url"`
	ServerName    string `json:"server_name"`
	Provisioned   bool   `json:"provisioned"`
}

type matrixProvisionResult struct {
	HomeserverURL string
	ServerName    string
}

func loadMatrixProvisionState(dataDir string) (*matrixProvisionState, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, matrixProvisionFile))
	if err != nil {
		return nil, err
	}
	var state matrixProvisionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func saveMatrixProvisionState(dataDir string, state *matrixProvisionState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, matrixProvisionFile), data, 0600)
}

// requestMatrixProvision calls the SaaS proxy API to register a Matrix user
// for this standalone node. The node sends its HKDF-derived password (from
// privKey); SaaS uses its Synapse admin credentials to do the actual registration.
func requestMatrixProvision(saasAPIURL, apiKey, peerID string, privKey crypto.PrivKey) (*matrixProvisionResult, error) {
	privKeyBytes, err := privKey.Raw()
	if err != nil {
		return nil, fmt.Errorf("extract private key bytes: %w", err)
	}

	passwordHash, err := encryption.DeriveMatrixPassword(privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("derive matrix password: %w", err)
	}

	reqBody, _ := json.Marshal(map[string]string{
		"password_hash": passwordHash,
	})

	endpoint := strings.TrimRight(saasAPIURL, "/") + "/platform/v1/stores/" + url.PathEscape(peerID) + "/matrix/provision"
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Standalone-Store-Key", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.NewDecoder(resp.Body).Decode(&errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("provision API returned %d: %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("provision API returned %d", resp.StatusCode)
	}

	var apiResp struct {
		Data struct {
			HomeserverURL string `json:"homeserver_url"`
			ServerName    string `json:"server_name"`
			Provisioned   bool   `json:"provisioned"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !apiResp.Data.Provisioned {
		return nil, fmt.Errorf("provision API returned provisioned=false")
	}

	return &matrixProvisionResult{
		HomeserverURL: apiResp.Data.HomeserverURL,
		ServerName:    apiResp.Data.ServerName,
	}, nil
}
