package cmd

import (
	"context"
	"fmt"
	"github.com/cpacia/openbazaar3.0/core"
	"github.com/cpacia/openbazaar3.0/events"
	"github.com/cpacia/openbazaar3.0/repo"
	"github.com/cpacia/openbazaar3.0/version"
	"github.com/fatih/color"
	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/op/go-logging"
	"os"
	"os/signal"
	"sort"
)

var log = logging.MustGetLogger("CMD")

// Start is the main entry point for openbazaar-go. The options to this
// command are the same as the OpenBazaar node config options.
type Start struct {
	repo.Config
}

// Execute starts the OpenBazaar node.
func (x *Start) Execute(args []string) error {
	cfg, _, err := repo.LoadConfig()
	if err != nil {
		return err
	}

	n, err := core.NewNode(context.Background(), cfg)
	if err != nil {
		return err
	}
	printSplashScreen()
	log.Infof("PeerID: %s", n.Identity())
	n.Start()
	printSwarmAddrs(n.IPFSNode())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	for range c {
		switch n.Stop(false) {
		case core.ErrPublishingActive:
			sub, err := n.SubscribeEvent(&events.PublishFinished{})
			if err != nil {
				return err
			}
			log.Info("OpenBazaar is currently publishing. Press ctl +c again to force shutdown.")
			select {
			case <-c:
			case <-sub.Out():
			}
			log.Info("OpenBazaar shutting down...")
			n.Stop(true)
			os.Exit(1)
		case core.ErrIPFSDelayedShutdown:
			sub, err := n.SubscribeEvent(&events.IPFSShutdown{})
			if err != nil {
				return err
			}
			log.Info("IPFS node is shutting down. Press ctl +c again to force shutdown.")
			select {
			case <-c:
			case <-sub.Out():
			}
			log.Info("OpenBazaar shutting down...")
			os.Exit(1)
		case nil:
			log.Info("OpenBazaar shutting down...")
			os.Exit(1)
		}
	}

	return nil
}

func printSwarmAddrs(node *ipfscore.IpfsNode) {
	var lisAddrs []string
	ifaceAddrs, err := node.PeerHost.Network().InterfaceListenAddresses()
	if err != nil {
		log.Errorf("failed to read listening addresses: %s", err)
	}
	for _, addr := range ifaceAddrs {
		lisAddrs = append(lisAddrs, addr.String())
	}
	sort.Strings(lisAddrs)
	for _, addr := range lisAddrs {
		fmt.Printf("Swarm listening on %s\n", addr)
	}

}

func printSplashScreen() {
	blue := color.New(color.FgBlue)
	white := color.New(color.FgWhite)

	for i, l := range []string{
		"________             ",
		"         __________",
		`\_____  \ ______   ____   ____`,
		`\______   \_____  _____________  _____ _______`,
		` /   |   \\____ \_/ __ \ /    \`,
		`|    |  _/\__  \ \___   /\__  \ \__  \\_  __ \ `,
		`/    |    \  |_> >  ___/|   |  \    `,
		`|   \ / __ \_/    /  / __ \_/ __ \|  | \/`,
		`\_______  /   __/ \___  >___|  /`,
		`______  /(____  /_____ \(____  (____  /__|`,
		`        \/|__|        \/     \/  `,
		`     \/      \/      \/     \/     \/`,
	} {
		if i%2 == 0 {
			if _, err := white.Printf(l); err != nil {
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
	fmt.Printf("\nopenbazaar-go v%s\n", version.String())
}
