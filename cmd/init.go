package cmd

import (
	"errors"
	"github.com/cpacia/multiwallet"
	"github.com/cpacia/openbazaar3.0/core"
	"github.com/cpacia/openbazaar3.0/repo"
	iwallet "github.com/cpacia/wallet-interface"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"os"
	"strings"
	"time"
)

// Init initializes a new OpenBazaar node at the provided path.
type Init struct {
	DataDir            string `short:"d" long:"datadir" description:"Directory to store data"`
	Mnemonic           string `short:"m" long:"mnemonic" description:"A mnemonic seed to initialize the node with"`
	Force              bool   `short:"f" long:"force" description:"Force overwrite existing repo (dangerous!)"`
	WalletCreationDate string `short:"w" long:"walletcreationdate" description:"Specify the date the seed was created. If omitted the wallet will sync from the oldest checkpoint."`
}

// Execute initializes the OpenBazaar node.
func (x *Init) Execute(args []string) error {
	if x.DataDir == "" {
		x.DataDir = repo.DefaultHomeDir
	}

	if fsrepo.IsInitialized(x.DataDir) && !x.Force {
		return errors.New("node is already initialized")
	}

	os.RemoveAll(x.DataDir)

	cfg, err := repo.LoadConfig()
	if err != nil {
		return err
	}

	var r *repo.Repo
	if x.Mnemonic != "" {
		r, err = repo.NewRepoWithCustomMnemonicSeed(x.DataDir, x.Mnemonic)
	} else {
		r, err = repo.NewRepo(x.DataDir)
	}

	enabledWallets := make([]iwallet.CoinType, len(cfg.EnabledWallets))
	for i, ew := range cfg.EnabledWallets {
		enabledWallets[i] = iwallet.CoinType(strings.ToUpper(ew))
	}

	opts := []multiwallet.Option{
		multiwallet.DataDir(cfg.DataDir),
		multiwallet.Wallets(enabledWallets),
	}
	mw, err := multiwallet.NewMultiwallet(opts...)
	if err != nil {
		return err
	}

	walletCreationDate := time.Now()
	if x.WalletCreationDate != "" {
		walletCreationDate, err = time.Parse(time.RFC3339, x.WalletCreationDate)
		if err != nil {
			return err
		}
	}

	if err := core.InitializeMultiwallet(mw, r.DB(), walletCreationDate); err != nil {
		return err
	}
	return err
}
