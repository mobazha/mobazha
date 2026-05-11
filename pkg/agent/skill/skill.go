package skill

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Tier indicates the protection level of a skill.
type Tier int

const (
	Tier0PlainText Tier = iota // public, embedded in binary
	Tier1Encrypted             // per-license envelope encryption
	Tier2Remote                // SaaS-only RPC execution
)

// Skill represents a loaded agent skill (recipe/playbook/prompt).
type Skill struct {
	ID          string            `json:"id"`
	Tier        Tier              `json:"tier"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Content     string            `json:"content"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Filter constrains which skills to list.
type Filter struct {
	Tier    *Tier
	Persona string
}

// Provider loads and lists available skills.
type Provider interface {
	Load(ctx context.Context, skillID string) (*Skill, error)
	List(ctx context.Context, filter Filter) ([]string, error)
}

// Common errors.
var (
	ErrSkillNotFound = errors.New("agent: skill not found")
	ErrDecryptFailed = errors.New("agent: skill decryption failed")
)

// FilesystemProvider loads Tier-0 plain text skills from a directory.
// Each skill is a subdirectory containing a SKILL.md file.
type FilesystemProvider struct {
	rootDir string
}

// NewFilesystemProvider creates a provider rooted at the given directory.
func NewFilesystemProvider(rootDir string) *FilesystemProvider {
	return &FilesystemProvider{rootDir: rootDir}
}

func (p *FilesystemProvider) Load(_ context.Context, skillID string) (*Skill, error) {
	cleanID := filepath.Clean(skillID)
	if strings.Contains(cleanID, "..") {
		return nil, ErrSkillNotFound
	}

	skillPath := filepath.Join(p.rootDir, cleanID, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSkillNotFound
		}
		return nil, err
	}

	return &Skill{
		ID:      cleanID,
		Tier:    Tier0PlainText,
		Name:    cleanID,
		Content: string(data),
	}, nil
}

func (p *FilesystemProvider) List(_ context.Context, filter Filter) ([]string, error) {
	if filter.Tier != nil && *filter.Tier != Tier0PlainText {
		return nil, nil
	}

	entries, err := os.ReadDir(p.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(p.rootDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			ids = append(ids, entry.Name())
		}
	}
	return ids, nil
}

// EncryptedProvider is a placeholder for Tier-1 envelope encryption.
// Implementation requires external cryptography audit (v1.1 mandate).
type EncryptedProvider struct{}

func (EncryptedProvider) Load(context.Context, string) (*Skill, error) {
	return nil, errors.New("EncryptedProvider not yet implemented")
}

func (EncryptedProvider) List(context.Context, Filter) ([]string, error) {
	return nil, errors.New("EncryptedProvider not yet implemented")
}

// RemoteResolver is a placeholder for Tier-2 SaaS RPC skill execution.
type RemoteResolver struct{}

func (RemoteResolver) Load(context.Context, string) (*Skill, error) {
	return nil, errors.New("RemoteResolver not yet implemented")
}

func (RemoteResolver) List(context.Context, Filter) ([]string, error) {
	return nil, errors.New("RemoteResolver not yet implemented")
}
