package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mobazha/mobazha/internal/repo"
)

type Config = repo.Config
type BrandFields = repo.BrandFields
type NetworkFields = repo.NetworkFields

// PlatformAIEndpointConfig is one platform-managed LLM endpoint route.
type PlatformAIEndpointConfig = repo.PlatformAIEndpointConfig

// PlatformAIProfileConfig groups platform-managed text and vision LLM routes.
type PlatformAIProfileConfig = repo.PlatformAIProfileConfig

func LoadConfig(homeDir string) (*Config, error) {
	return repo.LoadConfig(homeDir)
}

func AppDataDir(appName string, roaming bool) string {
	return repo.AppDataDir(appName, roaming)
}

const (
	DefaultNodeID           = repo.DefaultNodeID
	DefaultGatewayPort      = repo.DefaultGatewayPort
	DefaultGatewayMultiaddr = repo.DefaultGatewayMultiaddr
)

func IsInitialized(dataDir string) bool {
	return repo.IsRepoInitialized(dataDir)
}

// EnsureInitialized creates the shared application marker and one node
// repository. Distribution CLIs use this instead of importing internal repo
// bootstrap code or duplicating key-generation semantics.
func EnsureInitialized(dataDir, nodeID string, testnet bool) error {
	if repo.IsRepoInitialized(dataDir) {
		return nil
	}
	if nodeID == "" {
		nodeID = repo.DefaultNodeID
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create application data directory: %w", err)
	}
	nodePath := filepath.Join(dataDir, "nodes", nodeID)
	nodeRepo, err := repo.NewRepo(nodeID, nodePath, testnet)
	if err != nil {
		return fmt.Errorf("initialize node repository: %w", err)
	}
	nodeRepo.Close()
	version := strconv.Itoa(repo.DefaultRepoVersion)
	if err := os.WriteFile(filepath.Join(dataDir, "version"), []byte(version), 0o644); err != nil {
		return fmt.Errorf("write application repository version: %w", err)
	}
	return nil
}

func LoadBrandConfig(dataDir string) (*BrandFields, error) {
	return repo.LoadBrandConfig(dataDir)
}

func DefaultBrandName() string {
	return repo.DefaultBrandName()
}
