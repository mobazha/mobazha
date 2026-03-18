package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
)

const (
	adminPasswordLength = 24
	adminUsername        = "admin"
	passwordCharset     = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	adminPasswordFile   = "admin_password"
	adminHashFile       = "admin_password_hash"
	setupCompleteFile   = "setup_complete"
)

// HashFilePath returns the path to the admin password hash file within dataDir.
func HashFilePath(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, adminHashFile)
}

// PlainFilePath returns the path to the first-run plaintext password file within dataDir.
func PlainFilePath(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, adminPasswordFile)
}

// SetupCompleteFilePath returns the path to the setup_complete flag file within dataDir.
func SetupCompleteFilePath(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, setupCompleteFile)
}

// LoadCredentials implements the credential priority chain:
//  1. Hash file on disk (survives runtime password changes)
//  2. Config/CLI values (initial seed)
//
// Returns (username, passwordHash). Both empty if no credentials found.
func LoadCredentials(dataDir, configUsername, configPassword string) (string, string) {
	if dataDir != "" {
		hashPath := HashFilePath(dataDir)
		if hashBytes, err := os.ReadFile(hashPath); err == nil {
			hash := strings.TrimSpace(string(hashBytes))
			if hash != "" {
				username := configUsername
				if username == "" {
					username = adminUsername
				}
				return username, hash
			}
		}
	}

	return configUsername, configPassword
}

// GenerateAdminPassword creates a cryptographically random password using
// an unambiguous character set (no 0/O/l/1/I).
func GenerateAdminPassword() (string, error) {
	result := make([]byte, adminPasswordLength)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordCharset))))
		if err != nil {
			return "", fmt.Errorf("generating random password: %w", err)
		}
		result[i] = passwordCharset[n.Int64()]
	}
	return string(result), nil
}

// EnsureStandaloneAuth guarantees that admin credentials exist on disk and
// returns (username, passwordHashHex, generatedPlaintext, error).
// generatedPlaintext is non-empty only when a new password was created.
func EnsureStandaloneAuth(dataDir string) (string, string, string, error) {
	hashPath := HashFilePath(dataDir)
	plainPath := PlainFilePath(dataDir)

	// Case 1: hash file already exists — load it.
	if hashBytes, err := os.ReadFile(hashPath); err == nil {
		return adminUsername, strings.TrimSpace(string(hashBytes)), "", nil
	}

	// Case 2: plaintext file exists (e.g. set by Docker env via init-data) — hash and remove.
	if plainBytes, err := os.ReadFile(plainPath); err == nil {
		plain := strings.TrimSpace(string(plainBytes))
		h := sha256.Sum256([]byte(plain))
		hashHex := hex.EncodeToString(h[:])
		if err := os.WriteFile(hashPath, []byte(hashHex), 0600); err != nil {
			return "", "", "", fmt.Errorf("writing hash file: %w", err)
		}
		_ = os.Remove(plainPath)
		return adminUsername, hashHex, "", nil
	}

	// Case 3: first run — generate new password.
	password, err := GenerateAdminPassword()
	if err != nil {
		return "", "", "", err
	}

	if err := os.WriteFile(plainPath, []byte(password), 0600); err != nil {
		return "", "", "", fmt.Errorf("writing plaintext file: %w", err)
	}

	h := sha256.Sum256([]byte(password))
	hashHex := hex.EncodeToString(h[:])
	if err := os.WriteFile(hashPath, []byte(hashHex), 0600); err != nil {
		return "", "", "", fmt.Errorf("writing hash file: %w", err)
	}

	return adminUsername, hashHex, password, nil
}
