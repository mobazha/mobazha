package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRewriteMediaCDNToGateway(t *testing.T) {
	cidValue := "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"
	got := RewriteMediaCDNToGateway(
		"https://media.mobazha.org/"+cidValue,
		"https://app.mobazha.org",
	)
	want := "https://app.mobazha.org/v1/media/images/" + cidValue
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteMediaCDNToGateway_CustomCDN(t *testing.T) {
	cidValue := "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"
	got := RewriteMediaCDNToGateway(
		"https://cdn.example.net/cache/"+cidValue+"?v=1",
		"https://app.mobazha.org",
	)
	want := "https://app.mobazha.org/v1/media/images/" + cidValue
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteMediaCDNToGateway_IpfsPath(t *testing.T) {
	cidValue := "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"
	got := RewriteMediaCDNToGateway(
		"https://gateway.example.net/ipfs/"+cidValue,
		"https://app.mobazha.org",
	)
	want := "https://app.mobazha.org/v1/media/images/" + cidValue
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteMediaCDNToGateway_NonCIDPassthrough(t *testing.T) {
	raw := "https://cdn.example.net/photo.jpg"
	got := RewriteMediaCDNToGateway(raw, "https://app.mobazha.org")
	if got != raw {
		t.Fatalf("expected passthrough, got %q", got)
	}
}

func TestInlineImageURLs_PassthroughDataURL(t *testing.T) {
	dataURL := "data:image/jpeg;base64,/9j/4AAQ"
	out, err := InlineImageURLs(context.Background(), nil, "", false, []string{dataURL})
	if err != nil {
		t.Fatalf("InlineImageURLs: %v", err)
	}
	if out[0] != dataURL {
		t.Fatalf("expected passthrough, got %q", out[0])
	}
}

func TestInlineImageURLs_AllowsRewrittenLocalGatewayMedia(t *testing.T) {
	cidValue := "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/media/images/"+cidValue {
			t.Fatalf("unexpected fetch path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\npng-body"))
	}))
	defer server.Close()

	out, err := InlineImageURLs(
		context.Background(),
		server.Client(),
		server.URL,
		true,
		[]string{"https://cdn.example.net/" + cidValue},
	)
	if err != nil {
		t.Fatalf("InlineImageURLs: %v", err)
	}
	if !strings.HasPrefix(out[0], "data:image/png;base64,") {
		t.Fatalf("expected PNG data URL, got %q", out[0])
	}
}

func TestInlineImageURLs_BlocksDirectLocalhostURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("direct localhost URL should not be fetched")
	}))
	defer server.Close()

	_, err := InlineImageURLs(context.Background(), server.Client(), "", false, []string{server.URL + "/photo.png"})
	if err == nil {
		t.Fatal("expected direct localhost URL to be rejected")
	}
}

func TestInlineImageURLs_BlocksRewrittenLocalGatewayWithoutTrustedBypass(t *testing.T) {
	cidValue := "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("untrusted loopback gateway URL should not be fetched")
	}))
	defer server.Close()

	_, err := InlineImageURLs(
		context.Background(),
		server.Client(),
		server.URL,
		false,
		[]string{"https://cdn.example.net/" + cidValue},
	)
	if err == nil {
		t.Fatal("expected rewritten loopback gateway URL to be rejected")
	}
}

func TestInlineImageURLs_RejectsUnknownImageContent(t *testing.T) {
	cidValue := "QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("not an image"))
	}))
	defer server.Close()

	_, err := InlineImageURLs(
		context.Background(),
		server.Client(),
		server.URL,
		true,
		[]string{"https://cdn.example.net/" + cidValue},
	)
	if err == nil || !strings.Contains(err.Error(), "unsupported image content") {
		t.Fatalf("expected unsupported image content error, got %v", err)
	}
}
