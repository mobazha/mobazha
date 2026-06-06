//go:build !private_distribution

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"sort"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/mobazha/mobazha3.0/internal/api"
	"github.com/mobazha/mobazha3.0/internal/core"
	"github.com/mobazha/mobazha3.0/internal/embedded/frontend"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/version"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/logging"
)

var log = logging.MustGetLogger("CMD")

// Start is the main entry point for mobazha-go. The options to this
// command are the same as the Mobazha node config options.
type Start struct {
	repo.Config
	OpenBrowser bool `long:"open" description:"Automatically open the Web UI in the default browser after startup"`
}

// Execute starts the Mobazha node.
func (x *Start) Execute(args []string) error {
	cfg, err := repo.LoadConfig("")
	if err != nil {
		return err
	}

	// EXTERNAL_PAYMENTDaemonSeeds / EXTERNAL_PAYMENTSeedFile are only meaningful in the private_distribution build
	// (they feed the I2P-Only ExternalPayment NodePool). Reject them in full builds
	// so misconfiguration is caught at startup rather than silently ignored.
	if len(cfg.EXTERNAL_PAYMENTDaemonSeeds) > 0 {
		return fmt.Errorf("--external_paymentdaemonseeds is only supported in the private_distribution build")
	}
	if cfg.EXTERNAL_PAYMENTSeedFile != "" {
		return fmt.Errorf("--external_paymentseedfile is only supported in the private_distribution build")
	}

	if !repo.IsRepoInitialized(cfg.DataDir) {
		log.Info("Data directory not initialized, running first-time setup...")
		if err := autoInit(cfg); err != nil {
			return fmt.Errorf("auto-init failed: %w", err)
		}
		log.Info("Initialization complete.")
	}

	printSplashScreen()
	opts := []core.NodeOption{
		core.WithManagedEscrowCapConfig(cfg.ManagedEscrowCapabilityConfig()),
	}
	n, err := core.NewNodeWithOptions(context.Background(), cfg, repo.DefaultNodeID, nil, opts...)
	if err != nil {
		return err
	}
	log.Infof("PeerID: %s", n.Identity())
	n.Start()
	printSwarmAddrs(n.PeerHost())
	printReadyBanner(cfg)

	if x.OpenBrowser && frontend.HasContent() {
		openBrowser(gatewayURL(cfg))
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	for sig := range c {
		if sig == syscall.SIGTERM {
			log.Info("Mobazha killed")
			os.Exit(1)
		}
		switch n.Stop(false) {
		case coreiface.ErrPublishingActive:
			sub, err := n.SubscribeEvent(&events.PublishFinished{})
			if err != nil {
				return err
			}
			log.Info("Mobazha is currently publishing. Press ctl+c again to force shutdown.")
			select {
			case <-c:
			case <-sub.Out():
			}
			log.Info("Mobazha shutting down...")
			n.Stop(true)
			os.Exit(1)
		case coreiface.ErrP2PDelayedShutdown:
			sub, err := n.SubscribeEvent(&events.P2PShutdown{})
			if err != nil {
				return err
			}
			log.Info("P2P node is shutting down. Press ctl+c again to force shutdown.")
			select {
			case <-c:
			case <-sub.Out():
			}
			log.Info("Mobazha shutting down...")
			os.Exit(1)
		case nil:
			log.Info("Mobazha shutting down...")
			os.Exit(1)
		}
	}

	return nil
}

func printSwarmAddrs(h host.Host) {
	if h == nil {
		return
	}
	var lisAddrs []string
	ifaceAddrs, err := h.Network().InterfaceListenAddresses()
	if err != nil {
		log.Errorf("failed to read listening addresses: %s", err)
	}
	for _, addr := range ifaceAddrs {
		lisAddrs = append(lisAddrs, addr.String())
	}
	sort.Strings(lisAddrs)
	for _, addr := range lisAddrs {
		log.Infof("Swarm listening on %s\n", addr)
	}
}

func printReadyBanner(cfg *repo.Config) {
	apiURL := gatewayURL(cfg)

	green := color.New(color.FgGreen, color.Bold)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	green.Println("✅ Mobazha node is ready!")
	fmt.Println()

	if frontend.HasContent() {
		cyan.Printf("   Web UI:        %s\n", apiURL)
		cyan.Printf("   API endpoint:  %s/v1/\n", apiURL)
		fmt.Println()
		if cfg.DataDir != "" && !api.IsSetupComplete(cfg.DataDir) {
			fmt.Println("   Open the Web UI in your browser to set up your store.")
		} else {
			fmt.Println("   Open the Web UI in your browser to manage your store.")
		}
	} else {
		cyan.Printf("   API endpoint:  %s\n", apiURL)
		fmt.Println()
		fmt.Println("   This binary does not include the Web UI.")
		fmt.Println("   Use the Docker image for the full experience:")
		fmt.Println("   https://mobazha.org/self-host")
	}
	fmt.Println()
	fmt.Println("   Press Ctrl+C to stop the node.")
	fmt.Println("   To run as a background service: mobazha service install")
	fmt.Println()
}

func printSplashScreen() {
	blue := color.New(color.FgBlue)
	white := color.New(color.FgWhite)

	for i, l := range []string{
		`   _____        ___.                 .__            `,
		`  /     \   ____\_ |__ _____  _______|  |__ _____    `,
		` /  \ /  \ /  _ \| __ \\__  \ \___   /  |  \\__  \  `,
		`/    Y    (  <_> ) \_\ \/ __ \_/    /|   Y  \/ __ \_`,
		`\____|__  /\____/|___  (____  /_____ \___|  (____  /`,
		`        \/           \/     \/      \/    \/     \/ `,
	} {
		if i%2 == 0 {
			if _, err := white.Println(l); err != nil {
				log.Debug(err)
				return
			}
			continue
		}
		if _, err := blue.Println(l); err != nil {
			log.Debug(err)
			return
		}
	}

	blue.DisableColor()
	white.DisableColor()
	fmt.Println("")
	fmt.Printf("\nmobazha-go v%s\n", version.String())
}

func gatewayURL(cfg *repo.Config) string {
	gwAddr := cfg.GatewayAddr
	if gwAddr == "" {
		gwAddr = repo.DefaultGatewayMultiaddr
	}
	host, port := "127.0.0.1", repo.DefaultGatewayPort
	parts := strings.Split(gwAddr, "/")
	for i, p := range parts {
		switch p {
		case "ip4", "ip6":
			if i+1 < len(parts) {
				host = parts[i+1]
			}
		case "tcp", "udp":
			if i+1 < len(parts) {
				port = parts[i+1]
			}
		}
	}
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Warningf("Failed to open browser: %s", err)
	}
}

// autoInit performs a complete first-time repository initialization:
// creates the root data directory with version file, then initializes
// the default node repo (keys, DB, preferences) so that NewNode can
// find a fully-formed repository on startup.
func autoInit(cfg *repo.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return err
	}

	nodeRepoPath := path.Join(cfg.DataDir, "nodes", repo.DefaultNodeID)
	r, err := repo.NewRepo(repo.DefaultNodeID, nodeRepoPath, cfg.Testnet)
	if err != nil {
		return fmt.Errorf("repo init failed: %w", err)
	}
	r.Close()

	versionStr := fmt.Sprintf("%d", repo.DefaultRepoVersion)
	return os.WriteFile(
		path.Join(cfg.DataDir, "version"),
		[]byte(versionStr),
		0644,
	)
}
