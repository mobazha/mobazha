package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	l.Close()
	return addr
}

func TestServer_HTTPOnly_ServesAPIAndFrontend(t *testing.T) {
	httpAddr := freePort(t)

	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":"api"}`)
	})

	srv := New(Config{
		HTTPAddr:   httpAddr,
		APIHandler: apiHandler,
		DataDir:    t.TempDir(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = srv.ListenAndServe(ctx) }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get("http://" + httpAddr + "/v1/test")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "api")

	resp2, err := http.Get("http://" + httpAddr + "/healthz")
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

func TestServer_SelfSignedTLS(t *testing.T) {
	httpAddr := freePort(t)
	httpsAddr := freePort(t)

	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":"secure"}`)
	})

	srv := New(Config{
		HTTPAddr:   httpAddr,
		HTTPSAddr:  httpsAddr,
		APIHandler: apiHandler,
		DataDir:    t.TempDir(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = srv.ListenAndServe(ctx) }()
	time.Sleep(500 * time.Millisecond)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get("https://" + httpsAddr + "/v1/test")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "secure")
}

func TestServer_HTTPRedirectsToHTTPS(t *testing.T) {
	httpAddr := freePort(t)
	httpsAddr := freePort(t)

	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	srv := New(Config{
		HTTPAddr:   httpAddr,
		HTTPSAddr:  httpsAddr,
		APIHandler: apiHandler,
		DataDir:    t.TempDir(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = srv.ListenAndServe(ctx) }()
	time.Sleep(500 * time.Millisecond)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get("http://" + httpAddr + "/v1/test")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Location"), "https://")
}
