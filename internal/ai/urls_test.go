package ai

import "testing"

func TestResolveImageURLs(t *testing.T) {
	origin := "https://app.mobazha.org"
	got := ResolveImageURLs([]string{
		"/v1/media/images/QmTest",
		"https://media.mobazha.org/QmOther",
	}, origin)
	if got[0] != "https://app.mobazha.org/v1/media/images/QmTest" {
		t.Fatalf("relative URL = %q", got[0])
	}
	if got[1] != "https://media.mobazha.org/QmOther" {
		t.Fatalf("absolute URL = %q", got[1])
	}
}
