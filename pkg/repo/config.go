package repo

import "github.com/mobazha/mobazha3.0/internal/repo"

type Config = repo.Config

func LoadConfig(homeDir string) (*Config, error) {
	return repo.LoadConfig(homeDir)
}

func AppDataDir(appName string, roaming bool) string {
	return repo.AppDataDir(appName, roaming)
}

const DefaultNodeID = repo.DefaultNodeID
