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

func TestFilesystemProvider_LoadFrontMatterSkillMDRecursive(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "private", "product.import")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: product.import
description: Import local product materials into reviewable drafts.
persona: seller
capabilities:
  - listing.read
  - listing.draft_write
  - listing.apply_after_approval
tool_hints:
  - listings_get_template
  - listings_create
examples:
  - 批量导入商品 CSV
  - import product csv
risk: write_requires_approval
---

# Product Import

Use this private skill only for approved sellers.
`
	if err := os.WriteFile(filepath.Join(skillDir, "skill.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewFilesystemProvider(dir)
	s, err := p.Load(context.Background(), "product.import")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "product.import" || s.Name != "product.import" {
		t.Fatalf("unexpected skill identity: %#v", s)
	}
	if s.Description != "Import local product materials into reviewable drafts." {
		t.Fatalf("unexpected description: %q", s.Description)
	}
	if s.Metadata["risk"] != "write_requires_approval" || s.Metadata["persona"] != "seller" {
		t.Fatalf("unexpected metadata: %#v", s.Metadata)
	}
	if s.Persona() != "seller" {
		t.Fatalf("unexpected persona: %q", s.Persona())
	}
	if got := s.Capabilities(); len(got) != 3 || got[0] != "listing.read" || got[2] != "listing.apply_after_approval" {
		t.Fatalf("unexpected capabilities: %#v", got)
	}
	if got := s.ToolHints(); len(got) != 2 || got[0] != "listings_get_template" || got[1] != "listings_create" {
		t.Fatalf("unexpected tool hints: %#v", got)
	}
	if got := s.Examples(); len(got) != 2 || got[0] != "批量导入商品 CSV" || got[1] != "import product csv" {
		t.Fatalf("unexpected examples: %#v", got)
	}
	if s.Content == "" || s.Content[0:1] != "#" {
		t.Fatalf("expected body without frontmatter, got %q", s.Content)
	}
	if s.Location != "private/product.import/skill.md" {
		t.Fatalf("unexpected location: %s", s.Location)
	}
}

func TestFilesystemProvider_ResolveByLocationAliasAndPersonaFilter(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		path    string
		persona string
	}{
		{"seller/product.import/skill.md", "seller"},
		{"ops/workspace.daily-review/SKILL.md", "ops"},
	} {
		full := filepath.Join(dir, filepath.FromSlash(tc.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + filepath.Base(filepath.Dir(full)) + "\ndescription: desc\npersona: " + tc.persona + "\n---\nbody"
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	p := NewFilesystemProvider(dir)
	s, err := p.Load(context.Background(), "workspace.daily-review")
	if err != nil {
		t.Fatal(err)
	}
	if s.Location != "ops/workspace.daily-review/SKILL.md" {
		t.Fatalf("resolved wrong skill: %#v", s)
	}

	ids, err := p.List(context.Background(), Filter{Persona: "seller"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "product.import" {
		t.Fatalf("expected seller skill only, got %#v", ids)
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

func TestMultiProvider_PrivateProviderTakesPrecedence(t *testing.T) {
	publicDir := t.TempDir()
	privateDir := t.TempDir()
	writeSkill(t, publicDir, "product.import", "public")
	writeSkill(t, privateDir, "product.import", "private")

	p := NewMultiProvider(NewFilesystemProvider(privateDir), NewFilesystemProvider(publicDir))
	s, err := p.Load(context.Background(), "product.import")
	if err != nil {
		t.Fatal(err)
	}
	if s.Metadata["source"] != "private" {
		t.Fatalf("private provider should win, got %#v", s.Metadata)
	}
	ids, err := p.List(context.Background(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "product.import" {
		t.Fatalf("expected deduped skill list, got %#v", ids)
	}
}

func writeSkill(t *testing.T, root, id, source string) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + id + "\ndescription: desc\nsource: " + source + "\n---\nbody"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
