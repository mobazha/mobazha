package skill

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillRouter_PreselectsFromManifestExamples(t *testing.T) {
	dir := t.TempDir()
	writeSkillWithExamples(t, dir, "product.import", "seller", []string{
		"批量导入商品 CSV",
		"import product csv",
		"importar productos desde Excel",
	})
	writeSkillWithExamples(t, dir, "workspace.daily-review", "seller", []string{
		"review today's orders and messages",
	})

	router := NewSkillRouter(NewFilesystemProvider(dir))
	decision, err := router.Route(context.Background(), RouteInput{
		Text:   "importar productos desde Excel",
		Filter: Filter{Persona: "seller"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(decision.RequestedSkills) != 1 || decision.RequestedSkills[0] != "product.import" {
		t.Fatalf("expected product.import, got %#v", decision)
	}
	if len(decision.Preselected) != 1 || decision.Preselected[0].Source != "example" {
		t.Fatalf("expected example preselection, got %#v", decision.Preselected)
	}
}

func TestSkillRouter_PreselectsFromExplicitSkillNameInRequest(t *testing.T) {
	dir := t.TempDir()
	writeSkillWithExamples(t, dir, "product.import", "seller", []string{"import product csv"})

	router := NewSkillRouter(NewFilesystemProvider(dir))
	decision, err := router.Route(context.Background(), RouteInput{
		Text:   "Create a product import draft from the attached image",
		Filter: Filter{Persona: "seller"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(decision.RequestedSkills) != 1 || decision.RequestedSkills[0] != "product.import" {
		t.Fatalf("expected product.import, got %#v", decision)
	}
	if len(decision.Preselected) != 1 || decision.Preselected[0].Source != "name" {
		t.Fatalf("expected name preselection, got %#v", decision.Preselected)
	}
}

func TestSkillRouter_SkipsAmbiguousMatches(t *testing.T) {
	dir := t.TempDir()
	writeSkillWithExamples(t, dir, "product.import", "seller", []string{"import product csv"})
	writeSkillWithExamples(t, dir, "catalog.import", "seller", []string{"import product csv"})

	router := NewSkillRouter(NewFilesystemProvider(dir))
	decision, err := router.Route(context.Background(), RouteInput{
		Text:   "import product csv",
		Filter: Filter{Persona: "seller"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(decision.RequestedSkills) != 0 || decision.SkippedReason != "ambiguous_match" {
		t.Fatalf("expected ambiguous skip, got %#v", decision)
	}
}

func TestSkillRouter_ExplicitRequestWins(t *testing.T) {
	dir := t.TempDir()
	writeSkillWithExamples(t, dir, "product.import", "seller", []string{"import product csv"})

	router := NewSkillRouter(NewFilesystemProvider(dir))
	decision, err := router.Route(context.Background(), RouteInput{
		Text:           "hello",
		ExplicitSkills: []string{"product.import"},
		Filter:         Filter{Persona: "seller"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(decision.RequestedSkills) != 1 || decision.RequestedSkills[0] != "product.import" {
		t.Fatalf("expected explicit skill, got %#v", decision)
	}
}

func TestSkillRouter_ExplicitRequestRespectsFilter(t *testing.T) {
	dir := t.TempDir()
	writeSkillWithExamples(t, dir, "product.import", "seller", []string{"import product csv"})

	router := NewSkillRouter(NewFilesystemProvider(dir))
	decision, err := router.Route(context.Background(), RouteInput{
		Text:           "import product csv",
		ExplicitSkills: []string{"product.import"},
		Filter:         Filter{Persona: "buyer"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(decision.RequestedSkills) != 0 || decision.SkippedReason != "no_available_skills" {
		t.Fatalf("expected filter to block explicit skill, got %#v", decision)
	}
}

func TestSkillRouter_SkipsWeakMatches(t *testing.T) {
	dir := t.TempDir()
	writeSkillWithExamples(t, dir, "product.import", "seller", []string{"import product csv"})

	router := NewSkillRouter(NewFilesystemProvider(dir))
	decision, err := router.Route(context.Background(), RouteInput{
		Text:   "show product analytics",
		Filter: Filter{Persona: "seller"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(decision.RequestedSkills) != 0 || decision.SkippedReason != "no_confident_match" {
		t.Fatalf("expected weak match skip, got %#v", decision)
	}
}

func writeSkillWithExamples(t *testing.T, root, id, persona string, examples []string) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + id + "\npersona: " + persona + "\nexamples:\n"
	for _, example := range examples {
		body += "  - " + example + "\n"
	}
	body += "---\n# " + id + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
