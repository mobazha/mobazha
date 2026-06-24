package api

import (
	"net/http"
	"testing"
)

func TestPublicRequestOrigin(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{
			name: "forwarded proto",
			req: &http.Request{
				Host:   "app.mobazha.org",
				Header: http.Header{"X-Forwarded-Proto": []string{"https"}},
			},
			want: "https://app.mobazha.org",
		},
		{
			name: "forwarded host",
			req: &http.Request{
				Host: "internal:5102",
				Header: http.Header{
					"X-Forwarded-Proto": []string{"https"},
					"X-Forwarded-Host":  []string{"store.example.com"},
				},
			},
			want: "https://store.example.com",
		},
		{
			name: "comma separated forwarded values",
			req: &http.Request{
				Host: "internal:5102",
				Header: http.Header{
					"X-Forwarded-Proto": []string{"https, http"},
					"X-Forwarded-Host":  []string{"store.example.com, internal:5102"},
				},
			},
			want: "https://store.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := publicRequestOrigin(tt.req); got != tt.want {
				t.Fatalf("publicRequestOrigin = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAllowLoopbackGatewayForRequest(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		remoteAddr string
		header     http.Header
		want       bool
	}{
		{
			name:       "direct localhost",
			host:       "localhost:8080",
			remoteAddr: "127.0.0.1:52233",
			header:     make(http.Header),
			want:       true,
		},
		{
			name:       "forwarded localhost is not trusted",
			host:       "app.mobazha.org",
			remoteAddr: "127.0.0.1:52233",
			header: http.Header{
				"X-Forwarded-Host": []string{"localhost:9999"},
				"X-Forwarded-For":  []string{"203.0.113.10"},
			},
			want: false,
		},
		{
			name:       "remote localhost host header is not trusted",
			host:       "localhost:8080",
			remoteAddr: "203.0.113.10:52233",
			header:     make(http.Header),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{
				Host:       tt.host,
				RemoteAddr: tt.remoteAddr,
				Header:     tt.header,
			}
			if got := allowLoopbackGatewayForRequest(r); got != tt.want {
				t.Fatalf("allowLoopbackGatewayForRequest = %v, want %v", got, tt.want)
			}
		})
	}
}
