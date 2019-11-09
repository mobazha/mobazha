package cmd

import (
	"errors"
	"github.com/cpacia/openbazaar3.0/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

// Init initializes a new OpenBazaar node at the provided path.
type Init struct {
	DataDir            string `short:"d" long:"datadir" description:"Directory to store data"`
	Mnemonic           string `short:"m" long:"mnemonic" description:"A mnemonic seed to initialize the node with"`
	Force              bool   `short:"f" long:"force" description:"force overwrite existing repo (dangerous!)"`
	WalletCreationDate string `short:"w" long:"walletcreationdate" description:"specify the date the seed was created. if omitted the wallet will sync from the oldest checkpoint."`
}

// Execute initializes the OpenBazaar node.
func (x *Init) Execute(args []string) error {
	if x.DataDir == "" {
		x.DataDir = repo.DefaultHomeDir
	}

	if !fsrepo.IsInitialized(x.DataDir) && !x.Force {
		return errors.New("node is already initialized")
	}

	var err error
	if x.Mnemonic != "" {
		_, err = repo.NewRepoWithCustomMnemonicSeed(x.DataDir, x.Mnemonic)
	} else {
		_, err = repo.NewRepo(x.DataDir)
	}

	// TODO: initialize multiwallet with birthdate.
	return err
}
