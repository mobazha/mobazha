package repo

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/mobazha/mobazha3.0/internal/common"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/ffsqlite"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/version"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/op/go-logging"
	"github.com/tyler-smith/go-bip39"

	ft "github.com/ipfs/boxo/ipld/unixfs"
	nsys "github.com/ipfs/boxo/namesys"
	goPath "github.com/ipfs/boxo/path"
	mfsr "github.com/ipfs/fs-repo-migrations/tools/mfsr"
	ci "github.com/libp2p/go-libp2p/core/crypto"
)

const (
	// DefaultRepoVersion is the current repo version used for migrations.
	DefaultRepoVersion = 5

	// versionFileName is the name of the version file.
	versionFileName = "version"

	// defaultMispaymentBuffer is the default buffer to use when calculating a
	// mispayment.
	defaultMispaymentBuffer = 1.0

	DefaultNodeID = "default"

	// Directory names
	IPFSDirName         = "ipfs"
	MobazhaFilesDirName = "mobazha-files"
)

var log = logging.MustGetLogger("REPO")

// Repo is a representation of an Mobazha data directory.
// In this we store:
// - IPFS node data
// - The mobazha.conf file
// - The node's data root directory
// - The Mobazha database
// - A wallet directory which holds wallet plugin data
type Repo struct {
	db      database.Database
	dataDir string
}

// NewRepo returns a new Repo for the given data directory. It will
// be initialized if it is not already.
func NewRepo(nodeID string, dataDir string, testnet bool) (*Repo, error) {
	return newRepo(nodeID, dataDir, "", false, testnet)
}

// NewRepoWithCustomMnemonicSeed behaves the same as NewRepo but allows
// the caller to pass in a custom mnemonic seed. This is usuful for
// restoring a node from seed.
func NewRepoWithCustomMnemonicSeed(nodeID string, dataDir, mnemonic string, testnet bool) (*Repo, error) {
	return newRepo(nodeID, dataDir, mnemonic, false, testnet)
}

// DB returns the database implementation.
func (r *Repo) DB() database.Database {
	return r.db
}

// DataDir returns the data directory associated with this repo.
func (r *Repo) DataDir() string {
	return r.dataDir
}

// Close will close the repo and associated databases.
func (r *Repo) Close() {
	r.db.Close()
}

// DestroyRepo deletes the entire directory. Do NOT use this unless you are
// positive you want to wipe all data.
func (r *Repo) DestroyRepo() error {
	if err := r.db.Close(); err != nil {
		return err
	}
	return os.RemoveAll(r.dataDir)
}

// ReadVersion reads the version number from file.
func (r *Repo) ReadVersion() (int, error) {
	fileContent, err := os.ReadFile(path.Join(r.dataDir, versionFileName))
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(fileContent))
}

// WriteVersion writes the version number to file.
func (r *Repo) WriteVersion(version int) error {
	versionStr := strconv.Itoa(version)
	return os.WriteFile(path.Join(r.dataDir, versionFileName), []byte(versionStr), os.ModePerm)
}

func newRepo(nodeID string, dataDir, mnemonicSeed string, inMemoryDB bool, testnet bool) (*Repo, error) {
	var (
		dbIdentity, dbEscrowKey, dbRatingKey, dbBip44Key, dbMnemonic, torKey, dbSolKey *models.Key
		err                                                                            error
		isNew                                                                          bool
	)
	ipfsDir := path.Join(dataDir, IPFSDirName)

	// Install IPFS database plugins. This is guarded by a sync.Once.
	installDatabasePlugins(ipfsDir)

	if !fsrepo.IsInitialized(ipfsDir) {
		if err := checkWriteable(ipfsDir); err != nil {
			return nil, err
		}
		if mnemonicSeed == "" {
			mnemonicSeed, err = createMnemonic(bip39.NewEntropy, bip39.NewMnemonic)
			if err != nil {
				return nil, err
			}
		}

		identitySeed := bip39.NewSeed(mnemonicSeed, "Secret Passphrase")
		identityKey, err := IdentityKeyFromSeed(identitySeed, 0)
		if err != nil {
			return nil, err
		}

		identity, err := IdentityFromKey(identityKey)
		if err != nil {
			return nil, err
		}
		conf := mustDefaultConfig(testnet)
		conf.Identity = identity
		if err := fsrepo.Init(ipfsDir, conf); err != nil {
			return nil, err
		}

		if err := initializeIpnsKeyspace(ipfsDir, identityKey); err != nil {
			return nil, err
		}

		hdSeed := bip39.NewSeed(mnemonicSeed, "")
		escrowKey, ratingKey, bip44Key, solKey, err := CreateHDKeys(hdSeed)
		if err != nil {
			return nil, err
		}

		_, torPriv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}

		dbIdentity = &models.Key{
			Name:  "identity",
			Value: identityKey,
		}
		dbEscrowKey = &models.Key{
			Name:  "escrow",
			Value: escrowKey.Serialize(),
		}
		dbRatingKey = &models.Key{
			Name:  "ratings",
			Value: ratingKey.Serialize(),
		}
		dbBip44Key = &models.Key{
			Name:  "bip44",
			Value: []byte(bip44Key.String()),
		}
		dbSolKey = &models.Key{
			Name:  "solana",
			Value: []byte(*solKey),
		}
		dbMnemonic = &models.Key{
			Name:  "mnemonic",
			Value: []byte(mnemonicSeed),
		}
		torKey = &models.Key{
			Name:  "tor",
			Value: torPriv.Seed(),
		}
		if err := cleanIdentityFromConfig(ipfsDir); err != nil {
			return nil, err
		}
		isNew = true
	} else {
		// autoMigrateIPFSConfig(dataDir)

		ipfsRepo := mfsr.RepoPath(path.Join(dataDir, IPFSDirName))
		if err := ipfsRepo.CheckVersion("13"); err == nil {
			logger.LogInfoWithIDf(log, nodeID, "update IPFS version file from 13 to 14")
			if err = migrateIPFSRepoFrom13To14(dataDir); err == nil {
				if err = ipfsRepo.WriteVersion("14"); err != nil {
					logger.LogInfoWithIDf(log, nodeID, "failed to update version file to 14, %v", err)
				}
			} else {
				logger.LogInfoWithIDf(log, nodeID, "migration failed, %v", err)
			}
		} else if err := ipfsRepo.CheckVersion("14"); err == nil {
			// Nothing need to migrate, directly update the version to 16
			if err := ipfsRepo.WriteVersion("16"); err != nil {
				logger.LogInfoWithIDf(log, nodeID, "failed to update version file to 16, %v", err)
			}
		} else if err := ipfsRepo.CheckVersion("15"); err == nil {
			// Nothing need to migrate, directly update the version to 16
			if err := ipfsRepo.WriteVersion("16"); err != nil {
				logger.LogInfoWithIDf(log, nodeID, "failed to update version file to 16, %v", err)
			}
		}
	}

	var db database.Database
	if inMemoryDB {
		db, err = ffsqlite.NewFFMemoryDB(dataDir)
		if err != nil {
			return nil, err
		}
	} else {
		db, err = ffsqlite.NewFFSqliteDB(dataDir)
		if err != nil {
			return nil, err
		}
	}

	if err := autoMigrateDatabase(db); err != nil {
		return nil, err
	}

	err = db.Update(func(tx database.Tx) error {
		if dbIdentity != nil {
			if err := tx.Save(&dbIdentity); err != nil {
				return err
			}
		}
		if dbEscrowKey != nil {
			if err := tx.Save(&dbEscrowKey); err != nil {
				return err
			}
		}
		if dbRatingKey != nil {
			if err := tx.Save(&dbRatingKey); err != nil {
				return err
			}
		}
		if dbBip44Key != nil {
			if err := tx.Save(&dbBip44Key); err != nil {
				return err
			}
		}
		if dbMnemonic != nil {
			if err := tx.Save(&dbMnemonic); err != nil {
				return err
			}
		}
		if torKey != nil {
			if err := tx.Save(&torKey); err != nil {
				return err
			}
		}
		if dbSolKey != nil {
			if err := tx.Save(&dbSolKey); err != nil {
				return err
			}
		}
		if isNew {
			err := tx.Save(&models.UserPreferences{
				AutoConfirm:       true,
				MisPaymentBuffer:  defaultMispaymentBuffer,
				ShowNsfw:          true,
				ShowNotifications: true,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := CheckAndSetUlimit(); err != nil {
		return nil, err
	}

	r := &Repo{
		dataDir: dataDir,
		db:      db,
	}
	if isNew {
		if err := r.WriteVersion(DefaultRepoVersion); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func SaveKeysToDB(tx database.Tx, escrowKey *btcec.PrivateKey, bip44Key *hdkeychain.ExtendedKey, solKey *ed25519.PrivateKey, ratingKey *btcec.PrivateKey) error {
	if err := tx.Save(&models.Key{
		Name:  "escrow",
		Value: escrowKey.Serialize(),
	}); err != nil {
		return fmt.Errorf("保存托管密钥失败: %v", err)
	}

	if err := tx.Save(&models.Key{
		Name:  "ratings",
		Value: ratingKey.Serialize(),
	}); err != nil {
		return fmt.Errorf("保存评分密钥失败: %v", err)
	}

	if err := tx.Save(&models.Key{
		Name:  "bip44",
		Value: []byte(bip44Key.String()),
	}); err != nil {
		return fmt.Errorf("保存BIP44密钥失败: %v", err)
	}

	if err := tx.Save(&models.Key{
		Name:  "solana",
		Value: []byte(*solKey),
	}); err != nil {
		return fmt.Errorf("保存SOL密钥失败: %v", err)
	}

	return nil
}

func GetKeysFromDB(tx database.Tx) (dbEscrowKey, dbBip44Key, dbSolKey, dbRatingKey models.Key, err error) {
	var escrowKey, ratingKey, bip44Key, solKey models.Key
	err = func() error {
		if err := tx.Read().Where("name = ?", "escrow").First(&escrowKey).Error; err != nil {
			return fmt.Errorf("获取托管密钥失败: %v", err)
		}

		if err := tx.Read().Where("name = ?", "ratings").First(&ratingKey).Error; err != nil {
			return fmt.Errorf("获取评分密钥失败: %v", err)
		}

		if err := tx.Read().Where("name = ?", "bip44").First(&bip44Key).Error; err != nil {
			return fmt.Errorf("获取BIP44密钥失败: %v", err)
		}

		if err := tx.Read().Where("name = ?", "solana").First(&solKey).Error; err != nil {
			return fmt.Errorf("获取SOL密钥失败: %v", err)
		}

		return nil
	}()
	if err != nil {
		return models.Key{}, models.Key{}, models.Key{}, models.Key{}, err
	}

	return escrowKey, bip44Key, solKey, ratingKey, nil
}

func (r *Repo) WriteUserAgent(comment string) error {
	return os.WriteFile(path.Join(r.db.PublicDataPath(), "user_agent"), []byte(fmt.Sprintf("%s%s", version.UserAgent(), comment)), os.ModePerm)
}

func checkWriteable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		// Directory exists, make sure we can write to it
		testfile := path.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("%s is not writeable by the current user", dir)
			}
			return fmt.Errorf("unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// Directory does not exist, check that we can create it
		return os.MkdirAll(dir, 0775)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s, incorrect permissions", err)
	}

	return err
}

func createMnemonic(newEntropy func(int) ([]byte, error), newMnemonic func([]byte) (string, error)) (string, error) {
	entropy, err := newEntropy(128)
	if err != nil {
		return "", err
	}
	mnemonic, err := newMnemonic(entropy)
	if err != nil {
		return "", err
	}
	return mnemonic, nil
}

func CreateHDKeys(seed []byte) (escrowKey, ratingKey *btcec.PrivateKey, bip44Key *hdkeychain.ExtendedKey, solKey *ed25519.PrivateKey, err error) {
	masterPrivKey, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	twoZeroNine, err := masterPrivKey.Derive(hdkeychain.HardenedKeyStart + 209)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	bip44Key, err = masterPrivKey.Derive(hdkeychain.HardenedKeyStart + 44)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	escrowHDKey, err := twoZeroNine.Derive(0)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	ratingHDKey, err := twoZeroNine.Derive(1)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	escrowKey, err = escrowHDKey.ECPrivKey()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	ratingKey, err = ratingHDKey.ECPrivKey()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// 从相同的 seed 生成 SOL 私钥
	solPriv := ed25519.NewKeyFromSeed(seed[:32]) // 使用 seed 的前 32 字节作为 SOL 私钥

	return escrowKey, ratingKey, bip44Key, &solPriv, nil
}

// InitializeKeyspace sets the ipns record for the given key to
// point to an empty directory.
func InitializeKeyspace(n *core.IpfsNode, key ci.PrivKey) error {
	ctx, cancel := context.WithCancel(n.Context())
	defer cancel()

	emptyDir := ft.EmptyDirNode()

	err := n.Pinning.Pin(ctx, emptyDir, false, "")
	if err != nil {
		return err
	}

	err = n.Pinning.Flush(ctx)
	if err != nil {
		return err
	}

	pub := nsys.NewIPNSPublisher(n.Routing, n.Repo.Datastore())

	return pub.Publish(ctx, key, goPath.FromCid(emptyDir.Cid()))
}

func initializeIpnsKeyspace(repoRoot string, privKeyBytes []byte) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	cfg, err := r.Config()
	if err != nil {
		return err
	}
	identity, err := IdentityFromKey(privKeyBytes)
	if err != nil {
		return err
	}

	cfg.Identity = identity

	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	return InitializeKeyspace(nd, nd.PrivateKey)
}

func mustDefaultConfig(testnet bool) *config.Config {
	bootstrapPeers, err := config.ParseBootstrapPeers([]string{}) // TODO:
	if err != nil {
		// BootstrapAddressesDefault are local and should never panic
		panic(err)
	}

	conf, err := config.Init(&dummyWriter{}, 4096)
	if err != nil {
		panic(err)
	}
	conf.Ipns.RecordLifetime = "720h"
	conf.Discovery.MDNS.Enabled = true
	conf.Addresses = config.Addresses{
		Swarm: []string{
			"/ip4/0.0.0.0/tcp/5101",
			"/ip6/::/tcp/5101",
			"/ip4/0.0.0.0/tcp/7105/ws",
			"/ip6/::/tcp/7105/ws",
		},
		API:     []string{""},
		Gateway: []string{"/ip4/127.0.0.1/tcp/5102"},
	}
	if testnet {
		conf.Addresses = config.Addresses{
			Swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip6/::/tcp/4001",
				"/ip4/0.0.0.0/tcp/9005/ws",
				"/ip6/::/tcp/9005/ws",
			},
			API:     []string{""},
			Gateway: []string{"/ip4/127.0.0.1/tcp/4002"},
		}
	}
	conf.Bootstrap = config.BootstrapPeerStrings(bootstrapPeers)
	conf.Swarm.EnableHolePunching = config.True
	conf.Swarm.RelayClient.Enabled = config.True
	conf.Swarm.ResourceMgr.Enabled = config.True

	return conf
}

type dummyWriter struct{}

func (d *dummyWriter) Write(p []byte) (n int, err error) { return 0, nil }

var pluginOnce sync.Once

// installDatabasePlugins installs the default database plugins
// used by mobazha-go. This function is guarded by a sync.Once
// so it isn't accidentally called more than once.
func installDatabasePlugins(ipfsDir string) {
	pluginOnce.Do(func() {
		loader, err := loader.NewPluginLoader(ipfsDir)
		if err != nil {
			panic(err)
		}
		err = loader.Initialize()
		if err != nil {
			panic(err)
		}

		err = loader.Inject()
		if err != nil {
			panic(err)
		}
	})
}

// This was used for IPFS with 4002 migration to 5102 in the beginning. No need now.
func autoMigrateIPFSConfig(dataDir string, testnet bool) error {
	if testnet {
		return nil
	}

	configPath := path.Join(dataDir, IPFSDirName, "config")
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Version 1.0.0 build uses 4002/4001/9005 ports, which are conflict with old ones.
	var oldCfg config.Config
	if err := json.Unmarshal(configFile, &oldCfg); err != nil {
		return err
	}
	if !strings.Contains(oldCfg.Addresses.Gateway[0], "4002") {
		return nil
	}

	var cfgIface interface{}
	if err := json.Unmarshal(configFile, &cfgIface); err != nil {
		return err
	}
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("invalid config file")
	}

	defaultConfig := mustDefaultConfig(testnet)
	cfg["Addresses"] = defaultConfig.Addresses

	out, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, os.ModePerm)
}

// The IPFS config file holds the private key to the node. First we aren't
// even using this key as we prefer to use one derived from a mnemonic, but
// second we don't want it sitting in the config file anyway. So this function
// removes the identity object from the config. The identity object will be
// added back into the config with the correct key/identity by the MobazhaNode
// builder.
func cleanIdentityFromConfig(dataDir string) error {
	configPath := path.Join(dataDir, "config")
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var cfgIface interface{}
	if err := json.Unmarshal(configFile, &cfgIface); err != nil {
		return err
	}
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("invalid config file")
	}
	delete(cfg, "Identity")
	out, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, os.ModePerm)
}

func autoMigrateDatabase(db database.Database) error {
	dbModels := []interface{}{
		&models.Key{},
		&models.CachedIPNSEntry{},
		&models.CachedIPNSRecord{},
		&models.OutgoingMessage{},
		&models.IncomingMessage{},
		&models.ChatMessage{},
		&models.ChatGroup{},
		&models.NotificationRecord{},
		&models.FollowerStat{},
		&models.FollowSequence{},
		&models.Coupon{},
		&models.Event{},
		&models.Order{},
		&models.TransactionMetadata{},
		&models.UserPreferences{},
		&models.StoreAndForwardServers{},
		&models.Case{},
		&models.Channel{},
		&models.StoreCartRecord{},
		&models.MatrixKeyBackup{},
		&models.MatrixCredentials{},
		&models.MatrixSecretsBundle{},
	}

	return db.Update(func(tx database.Tx) error {
		// 先迁移其他表
		for _, m := range dbModels {
			if err := tx.Migrate(m); err != nil {
				return fmt.Errorf("迁移表 %s 失败: %v", reflect.TypeOf(m).String(), err)
			}
		}

		// 特殊处理 ReceivingAccount 表
		// 检查表是否存在
		var count int64
		if err := tx.Read().Table("receiving_accounts").Count(&count).Error; err != nil {
			// 表不存在，直接创建
			if err := tx.Migrate(&models.ReceivingAccount{}); err != nil {
				return fmt.Errorf("迁移表 ReceivingAccount 失败: %v", err)
			}
		} else {
			// 检查表结构是否匹配
			type TableInfo struct {
				Cid       int    `gorm:"column:cid"`
				Name      string `gorm:"column:name"`
				Type      string `gorm:"column:type"`
				NotNull   int    `gorm:"column:notnull"`
				DfltValue string `gorm:"column:dflt_value"`
				Pk        int    `gorm:"column:pk"`
			}
			var columns []TableInfo
			if err := tx.Read().Raw("PRAGMA table_info(receiving_accounts)").Scan(&columns).Error; err != nil {
				return fmt.Errorf("获取表结构失败: %v", err)
			}

			// 检查是否需要迁移
			needsMigration := false
			columnMap := make(map[string]bool)
			for _, col := range columns {
				columnMap[col.Name] = true
			}

			// 检查新字段是否存在
			if !columnMap["serialized_active_tokens"] || !columnMap["serialized_inactive_tokens"] || !columnMap["is_active"] || !columnMap["status"] || !columnMap["created_at"] || !columnMap["updated_at"] {
				needsMigration = true
			}

			if needsMigration {
				// 删除旧表
				if err := tx.Read().Exec("DROP TABLE receiving_accounts").Error; err != nil {
					return fmt.Errorf("删除旧表失败: %v", err)
				}

				// 创建新表
				if err := tx.Migrate(&models.ReceivingAccount{}); err != nil {
					return fmt.Errorf("创建新表失败: %v", err)
				}
			}
		}

		return nil
	})
}

// CheckAndMigrateRepo checks and performs repository migrations
func CheckAndMigrateRepo(dataDir string) error {
	versionFile := path.Join(dataDir, versionFileName)
	fileContent, err := os.ReadFile(versionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // New repo, no need to migrate
		}
		return err
	}

	version, err := strconv.Atoi(string(fileContent))
	if err != nil {
		return err
	}

	// In version 5, the repo structure is changed to nodes structure.
	repoUpdateVersion := 5
	if version < repoUpdateVersion {
		log.Infof("Migrating repo from version %d to %d", version, repoUpdateVersion)
		if err := migrateRepoToNodesStructure(dataDir); err != nil {
			return fmt.Errorf("migration failed: %v", err)
		}
	}

	return nil
}

func migrateRepoToNodesStructure(rootPath string) error {
	defaultNodePath := path.Join(rootPath, "nodes", DefaultNodeID)

	if err := os.MkdirAll(defaultNodePath, 0755); err != nil {
		return fmt.Errorf("failed to create nodes directory structure: %v", err)
	}

	const oldMobazhaFilesDirName = "openbazaar-files"
	itemsToMove := []string{
		IPFSDirName,
		oldMobazhaFilesDirName,
		common.PublicDirName,
		common.DatabaseFileName,
		common.MultiwalletFileName,
		versionFileName,
	}

	for _, item := range itemsToMove {
		srcPath := path.Join(rootPath, item)
		dstPath := path.Join(defaultNodePath, item)
		if item == oldMobazhaFilesDirName {
			dstPath = path.Join(defaultNodePath, MobazhaFilesDirName)
		}

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}

		if err := os.Rename(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to move %s: %v", item, err)
		}
		log.Infof("Successfully moved %s to nodes structure", item)
	}

	return nil
}
