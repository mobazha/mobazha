package cmd

import (
	"fmt"
	"os"

	"github.com/mobazha/mobazha3.0/internal/repo"
)

type Status struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet bool   `short:"t" long:"testnet" description:"use the test network"`
}

func (x *Status) Execute(args []string) error {
	// Set repo path
	if x.DataDir == "" {
		x.DataDir = repo.DefaultHomeDir
		if x.Testnet {
			x.DataDir = repo.DefaultHomeDir + "-testnet"
		}
	}

	torAvailable := false
	isDBEncrypted := false

	if repo.IsRepoInitialized(x.DataDir) {

		if isDBEncrypted {
			if !torAvailable {
				fmt.Println("Initialized - Encrypted")
				os.Exit(30)
			} else {
				fmt.Println("Initialized - Encrypted")
				fmt.Println("Tor Available")
				os.Exit(31)
			}
		} else {
			if !torAvailable {
				fmt.Println("Initialized - Not Encrypted")
				os.Exit(20)
			} else {
				fmt.Println("Initialized - Not Encrypted")
				fmt.Println("Tor Available")
				os.Exit(21)
			}
		}
	} else {
		if !torAvailable {
			fmt.Println("Not initialized")
			os.Exit(10)
		} else {
			fmt.Println("Not initialized")
			fmt.Println("Tor Available")
			os.Exit(11)
		}
	}
	return nil
}
