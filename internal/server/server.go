package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mobazha/mobazha3.0/internal/embedded/frontend"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	"golang.org/x/crypto/acme/autocert"
)

var log = logging.MustGetLogger("SERVER")

// Config holds the unified HTTP/HTTPS server configuration.
type Config struct {
	// HTTPAddr is the address to listen on for HTTP (e.g. ":80" or ":8080").
	HTTPAddr string

	// HTTPSAddr is the address for HTTPS (e.g. ":443" or ":8443").
	// Empty string disables HTTPS.
	HTTPSAddr string

	// Domain for Let's Encrypt autocert. When set, ACME TLS is enabled.
	// Empty triggers self-signed certificate fallback.
	Domain string

	// DataDir is the base data directory for storing certificates.
	DataDir string

	// APIHandler is the http.Handler serving the /v1/ API routes.
	APIHandler http.Handler

	// FrontendOverrideDir, when set, serves frontend files from this
	// directory instead of the embedded SPA.
	FrontendOverrideDir string

	// PrivateDistributionMode, when true, enables extreme privacy headers (CSP,
	// no-store, no-referrer) and a stripped-down runtime-config.js payload.
	PrivateDistributionMode bool

	// Brand holds white-label overrides from brand.yaml. Passed to the
	// embedded frontend handler for /runtime-config.js injection.
	Brand *frontend.BrandSnapshot

	// FeaturesSnapshotFn, when set, is invoked per-request from the
	// embedded frontend's /runtime-config.js handler so toggles via
	// PUT /v1/settings/features/{key} or PATCH /platform/v1/features/{key}
	// propagate to the SPA without a process restart. A nil callback
	// yields an empty features map (fail-closed).
	FeaturesSnapshotFn func(context.Context) []frontend.FeatureSnapshot
}

// Server is the unified HTTP(S) server for the native binary distribution.
// It merges the API backend and embedded SPA frontend on a single port.
type Server struct {
	httpServer  *http.Server
	httpsServer *http.Server
	config      Config
	acmeManager *autocert.Manager
}

// New creates a unified server with the given configuration.
func New(cfg Config) *Server {
	feHandler := frontend.NewHandler(frontend.ServerConfig{
		OverrideDir: cfg.FrontendOverrideDir,
		Deployment: frontend.RuntimeDeployment{
			Mode: func() string {
				if cfg.PrivateDistributionMode {
					return frontend.RuntimeDeploymentPrivateDistribution
				}
				return frontend.RuntimeDeploymentStandalone
			}(),
		},
		Brand:              cfg.Brand,
		FeaturesSnapshotFn: cfg.FeaturesSnapshotFn,
	})

	mux := http.NewServeMux()

	mux.Handle("/v1/", cfg.APIHandler)
	mux.Handle("/ws/", cfg.APIHandler)
	mux.Handle("/ws", cfg.APIHandler)
	mux.Handle("/healthz", cfg.APIHandler)

	mux.Handle("/", feHandler)

	s := &Server{config: cfg}

	if cfg.HTTPSAddr != "" {
		var tlsCfg *tls.Config
		if cfg.Domain != "" {
			tlsCfg = s.autocertTLS(cfg)
		} else {
			tlsCfg = s.selfSignedTLS(cfg)
		}

		s.httpsServer = &http.Server{
			Addr:         cfg.HTTPSAddr,
			Handler:      mux,
			TLSConfig:    tlsCfg,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 120 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		s.httpServer = &http.Server{
			Addr:         cfg.HTTPAddr,
			Handler:      s.httpsRedirectHandler(),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
	} else {
		s.httpServer = &http.Server{
			Addr:         cfg.HTTPAddr,
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 120 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
	}

	return s
}

// ListenAndServe starts the server(s) and blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	errCh := make(chan error, 2)

	if s.httpsServer != nil {
		go func() {
			ln, err := net.Listen("tcp", s.httpsServer.Addr)
			if err != nil {
				errCh <- fmt.Errorf("HTTPS listen: %w", err)
				return
			}
			tlsLn := tls.NewListener(ln, s.httpsServer.TLSConfig)
			log.Infof("HTTPS server listening on %s", s.httpsServer.Addr)
			errCh <- s.httpsServer.Serve(tlsLn)
		}()
	}

	go func() {
		log.Infof("HTTP server listening on %s", s.httpServer.Addr)
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		s.shutdownAll()
		return err
	case <-ctx.Done():
		s.shutdownAll()
		return ctx.Err()
	}
}

func (s *Server) shutdownAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.httpsServer != nil {
		_ = s.httpsServer.Shutdown(ctx)
	}
	_ = s.httpServer.Shutdown(ctx)
}

func (s *Server) autocertTLS(cfg Config) *tls.Config {
	certDir := filepath.Join(cfg.DataDir, "certs")
	_ = os.MkdirAll(certDir, 0700)

	s.acmeManager = &autocert.Manager{
		Cache:      autocert.DirCache(certDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(cfg.Domain),
	}

	tlsCfg := s.acmeManager.TLSConfig()
	tlsCfg.MinVersion = tls.VersionTLS12
	return tlsCfg
}

func (s *Server) selfSignedTLS(cfg Config) *tls.Config {
	certDir := filepath.Join(cfg.DataDir, "certs")
	_ = os.MkdirAll(certDir, 0700)

	certFile := filepath.Join(certDir, "self-signed.crt")
	keyFile := filepath.Join(certDir, "self-signed.key")

	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		if err := generateSelfSignedCert(certFile, keyFile); err != nil {
			log.Errorf("Failed to generate self-signed cert: %v", err)
			return &tls.Config{MinVersion: tls.VersionTLS12}
		}
		log.Info("Generated self-signed TLS certificate")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Errorf("Failed to load self-signed cert: %v", err)
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
}

func (s *Server) httpsRedirectHandler() http.Handler {
	redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		target := "https://" + host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})

	if s.acmeManager != nil {
		return s.acmeManager.HTTPHandler(redirect)
	}
	return redirect
}

func generateSelfSignedCert(certFile, keyFile string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"Mobazha Standalone"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	return pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}
