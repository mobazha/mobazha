package skill

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesystemProvider_Load(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "test-skill")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\nContent here"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewFilesystemProvider(dir)

	s, err := p.Load(context.Background(), "test-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "test-skill" {
		t.Errorf("expected id test-skill, got %s", s.ID)
	}
	if s.Tier != Tier0PlainText {
		t.Errorf("expected Tier0, got %d", s.Tier)
	}
	if s.Content != "# Test Skill\nContent here" {
		t.Errorf("unexpected content: %s", s.Content)
	}
}

func TestFilesystemProvider_Load_NotFound(t *testing.T) {
	p := NewFilesystemProvider(t.TempDir())
	_, err := p.Load(context.Background(), "nonexistent")
	if !errors.Is(err, ErrSkillNotFound) {
		t.Errorf("expected ErrSkillNotFound, got %v", err)
	}
}

func TestFilesystemProvider_Load_PathTraversal(t *testing.T) {
	p := NewFilesystemProvider(t.TempDir())
	_, err := p.Load(context.Background(), "../../../etc/passwd")
	if !errors.Is(err, ErrSkillNotFound) {
		t.Errorf("expected ErrSkillNotFound for path traversal, got %v", err)
	}
}

func TestFilesystemProvider_List(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"skill-a", "skill-b"} {
		d := filepath.Join(dir, name)
		os.Mkdir(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("content"), 0o644)
	}
	// dir without SKILL.md should be excluded
	os.Mkdir(filepath.Join(dir, "not-a-skill"), 0o755)

	p := NewFilesystemProvider(dir)
	ids, err := p.List(context.Background(), Filter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 skills, got %d: %v", len(ids), ids)
	}
}

func TestFilesystemProvider_List_FilterTier(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "s1"), 0o755)
	os.WriteFile(filepath.Join(dir, "s1", "SKILL.md"), []byte("x"), 0o644)

	p := NewFilesystemProvider(dir)
	tier1 := Tier1Encrypted
	ids, err := p.List(context.Background(), Filter{Tier: &tier1})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Errorf("Tier1 filter should return no results from filesystem provider, got %v", ids)
	}
}

func TestEncryptedProvider_NotImplemented(t *testing.T) {
	p := EncryptedProvider{}
	_, err := p.Load(context.Background(), "any")
	if err == nil {
		t.Error("expected error from unimplemented provider")
	}
}

func TestRemoteResolver_NotImplemented(t *testing.T) {
	p := RemoteResolver{}
	_, err := p.Load(context.Background(), "any")
	if err == nil {
		t.Error("expected error from unimplemented provider")
	}
}
