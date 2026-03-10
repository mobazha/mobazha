package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/fatih/color"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/mobazha/mobazha3.0/internal/core"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/version"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("CMD")

// Start is the main entry point for mobazha-go. The options to this
// command are the same as the Mobazha node config options.
type Start struct {
	repo.Config
}

// Execute starts the Mobazha node.
func (x *Start) Execute(args []string) error {
	cfg, err := repo.LoadConfig("")
	if err != nil {
		return err
	}
	printSplashScreen()
	n, err := core.NewNode(context.Background(), cfg, repo.DefaultNodeID)
	if err != nil {
		return err
	}
	log.Infof("PeerID: %s", n.Identity())
	n.Start()
	printSwarmAddrs(n.PeerHost())

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
		case coreiface.ErrIPFSDelayedShutdown:
			sub, err := n.SubscribeEvent(&events.IPFSShutdown{})
			if err != nil {
				return err
			}
			log.Info("IPFS node is shutting down. Press ctl+c again to force shutdown.")
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
