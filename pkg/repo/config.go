package repo

import "github.com/mobazha/mobazha3.0/internal/repo"

type Config = repo.Config

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

const DefaultNodeID = repo.DefaultNodeID
