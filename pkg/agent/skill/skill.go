package skill

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	Location    string            `json:"location,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Persona returns the acting persona declared by the skill manifest.
func (s Skill) Persona() string {
	return strings.TrimSpace(s.Metadata["persona"])
}

// Capabilities returns the abstract business capabilities requested by this
// skill. They are requirements, not authorization grants.
func (s Skill) Capabilities() []string {
	return splitMetadataList(s.Metadata["capabilities"])
}

// ToolHints returns optional concrete tool hints for planning. Runtime policy
// must still authorize tools through the ToolCatalog before exposing them.
func (s Skill) ToolHints() []string {
	return splitMetadataList(s.Metadata["tool_hints"])
}

// Examples returns representative user requests that should activate this
// skill. Routers use examples as routing hints, not as security policy.
func (s Skill) Examples() []string {
	return splitMetadataList(s.Metadata["examples"])
}

// Modalities returns optional input modalities supported by this skill, such as
// text, csv, spreadsheet, image, or pdf.
func (s Skill) Modalities() []string {
	return splitMetadataList(s.Metadata["modalities"])
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

// LoadDefinitionFromString parses a Markdown skill definition. It is useful for
// embedded public reference skills and private providers that fetch Markdown
// from SaaS storage before handing it to the kernel.
func LoadDefinitionFromString(content, location string) (*Skill, error) {
	skill := parseMarkdownSkill(content, location)
	return &skill, nil
}

// FSProvider loads Tier-0 skills from an fs.FS. It is suitable for embedded
// skill packs as well as other read-only filesystem implementations.
type FSProvider struct {
	root fs.FS
}

// NewFSProvider creates a provider rooted at root.
func NewFSProvider(root fs.FS) *FSProvider {
	return &FSProvider{root: root}
}

func (p *FSProvider) Load(_ context.Context, skillID string) (*Skill, error) {
	requested := strings.TrimSpace(skillID)
	if requested == "" || strings.Contains(filepath.Clean(requested), "..") {
		return nil, ErrSkillNotFound
	}
	skills, err := p.discover()
	if err != nil {
		return nil, err
	}
	for i := range skills {
		if matches(requested, skills[i]) {
			cp := skills[i]
			return &cp, nil
		}
	}
	return nil, ErrSkillNotFound
}

func (p *FSProvider) List(_ context.Context, filter Filter) ([]string, error) {
	if filter.Tier != nil && *filter.Tier != Tier0PlainText {
		return nil, nil
	}
	skills, err := p.discover()
	if err != nil {
		return nil, err
	}
	return filterSkillIDs(skills, filter), nil
}

func (p *FSProvider) discover() ([]Skill, error) {
	if p == nil || p.root == nil {
		return nil, nil
	}
	return discoverSkills(p.root)
}

// FilesystemProvider loads Tier-0 plain text skills from Markdown files.
// It supports both WAE-style recursive skill.md files and Mobazha/Codex-style
// <skill-id>/SKILL.md directories.
type FilesystemProvider struct {
	rootDir string
}

// NewFilesystemProvider creates a provider rooted at the given directory.
func NewFilesystemProvider(rootDir string) *FilesystemProvider {
	return &FilesystemProvider{rootDir: rootDir}
}

func (p *FilesystemProvider) Load(_ context.Context, skillID string) (*Skill, error) {
	requested := strings.TrimSpace(skillID)
	if requested == "" || strings.Contains(filepath.Clean(requested), "..") {
		return nil, ErrSkillNotFound
	}
	skills, err := p.discover()
	if err != nil {
		return nil, err
	}
	for i := range skills {
		if matches(requested, skills[i]) {
			cp := skills[i]
			return &cp, nil
		}
	}
	return nil, ErrSkillNotFound
}

func (p *FilesystemProvider) List(_ context.Context, filter Filter) ([]string, error) {
	if filter.Tier != nil && *filter.Tier != Tier0PlainText {
		return nil, nil
	}
	skills, err := p.discover()
	if err != nil {
		return nil, err
	}
	return filterSkillIDs(skills, filter), nil
}

func (p *FilesystemProvider) discover() ([]Skill, error) {
	var skills []Skill
	if p == nil || p.rootDir == "" {
		return skills, nil
	}
	if _, err := os.Stat(p.rootDir); err != nil {
		if os.IsNotExist(err) {
			return skills, nil
		}
		return nil, err
	}
	return discoverSkills(os.DirFS(p.rootDir))
}

func discoverSkills(root fs.FS) ([]Skill, error) {
	var skills []Skill
	err := fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !isSkillFile(entry.Name()) {
			return nil
		}
		skill, err := loadMarkdownSkillFS(root, path)
		if err != nil {
			return err
		}
		skills = append(skills, skill)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].ID < skills[j].ID })
	return skills, nil
}

func filterSkillIDs(skills []Skill, filter Filter) []string {
	ids := make([]string, 0, len(skills))
	for _, skill := range skills {
		if filter.Persona != "" && skill.Metadata["persona"] != filter.Persona {
			continue
		}
		ids = append(ids, skill.ID)
	}
	sort.Strings(ids)
	return ids
}

func isSkillFile(name string) bool {
	return strings.EqualFold(name, "skill.md")
}

func loadMarkdownSkillFS(root fs.FS, path string) (Skill, error) {
	data, err := fs.ReadFile(root, path)
	if err != nil {
		return Skill{}, err
	}
	return parseMarkdownSkill(string(data), filepath.ToSlash(path)), nil
}

func parseMarkdownSkill(content, location string) Skill {
	id := skillIDFromPath(location)
	name := id
	description := ""
	metadata := map[string]string{}

	if fm, body, ok := splitFrontMatter(content); ok {
		content = strings.TrimSpace(body)
		for key, value := range fm {
			switch key {
			case "name":
				if value != "" {
					name = value
					id = value
				}
			case "description":
				description = value
			default:
				metadata[key] = value
			}
		}
	}
	return Skill{
		ID:          id,
		Tier:        Tier0PlainText,
		Name:        name,
		Description: description,
		Content:     content,
		Location:    filepath.ToSlash(location),
		Metadata:    metadata,
	}
}

func splitFrontMatter(content string) (map[string]string, string, bool) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil, content, false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, content, false
	}
	fm := map[string]string{}
	currentListKey := ""
	for i := 1; i < end; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if currentListKey != "" && strings.HasPrefix(line, "- ") {
			item := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "- ")), `"'`)
			if item != "" {
				if fm[currentListKey] != "" {
					fm[currentListKey] += "\n"
				}
				fm[currentListKey] += item
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			currentListKey = ""
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key != "" {
			fm[key] = value
			if value == "" {
				currentListKey = key
			} else {
				currentListKey = ""
			}
		}
	}
	return fm, strings.Join(lines[end+1:], "\n"), true
}

func skillIDFromPath(rel string) string {
	rel = filepath.ToSlash(rel)
	dir := filepath.Dir(rel)
	if dir == "." || dir == "" {
		return strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	}
	return filepath.Base(dir)
}

func matches(requested string, s Skill) bool {
	req := normalize(requested)
	return req == normalize(s.ID) ||
		req == normalize(s.Name) ||
		req == normalize(s.Location) ||
		req == normalize(skillIDFromPath(s.Location))
}

func normalize(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "\\", "/")
	return value
}

func splitMetadataList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t'
	})
	out := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		item := strings.Trim(strings.TrimSpace(field), `"'`)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
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
