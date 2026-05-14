package repo

import (
	"crypto/ed25519"
	"crypto/rand"
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
	"github.com/mobazha/mobazha3.0/internal/common"
	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	"github.com/mobazha/mobazha3.0/internal/version"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/tyler-smith/go-bip39"
	"gorm.io/gorm"
)

const (
	// DefaultRepoVersion is the current repo version used for migrations.
	DefaultRepoVersion = 6

	// versionFileName is the name of the version file.
	versionFileName = "version"

	// defaultMispaymentBuffer is the default buffer to use when calculating a
	// mispayment.
	defaultMispaymentBuffer = 1.0

	DefaultNodeID = "default"
)

var (
	log = logging.MustGetLogger("REPO")

	// sharedDBMigrateOnce ensures schema migration runs only once for the shared DB.
	// Multiple tenants may call NewRepoWithSharedDB concurrently, but DDL is global
	// and idempotent — running it once is sufficient.
	sharedDBMigrateOnce sync.Once
	sharedDBMigrateErr  error
)

// Repo is a representation of a Mobazha data directory.
// In this we store:
// - The mobazha.conf file
// - The node's data root directory
// - Keys and database
// - The Mobazha database
// - A wallet directory which holds wallet plugin data
type Repo struct {
	db      database.Database
	dataDir string
}

// NewRepo returns a new Repo for the given data directory. It will
// be initialized if it is not already.
func NewRepo(nodeID string, dataDir string, testnet bool) (*Repo, error) {
	return newRepo(nodeID, dataDir, "", nil, false, testnet)
}

// NewRepoWithCustomMnemonicSeed behaves the same as NewRepo but allows
// the caller to pass in a custom mnemonic seed. This is useful for
// restoring a node from seed.
func NewRepoWithCustomMnemonicSeed(nodeID string, dataDir, mnemonic string, testnet bool) (*Repo, error) {
	return newRepo(nodeID, dataDir, mnemonic, nil, false, testnet)
}

// NewRepoWithIdentityKey creates a new Repo using an externally provided identity key
// (in libp2p marshaled format). The mnemonic is still generated for wallet keys,
// but the identity key comes from KeyVault. If the repo already exists, the external
// identity key is stored/updated in the DB.
func NewRepoWithIdentityKey(nodeID string, dataDir string, identityKey []byte, testnet bool) (*Repo, error) {
	return newRepo(nodeID, dataDir, "", identityKey, false, testnet)
}

// NewRepoWithSharedDB creates a Repo backed by a shared *gorm.DB (multi-tenant mode).
// Instead of creating its own SQLite, it wraps the shared DB with a TenantDB that
// automatically scopes all queries to the given tenantID.
//
// If identityKey is provided (from KeyVault), it uses that key. Otherwise it checks
// the shared DB for existing keys.
//
// Note: Tor keys are NOT generated in shared-DB mode. SaaS tenant nodes communicate
// via SNF proxy and do not need Tor onion services. If Tor support is needed in the
// future, it should be added here alongside the other keys.
func NewRepoWithSharedDB(nodeID string, dataDir string, sharedDB *gorm.DB, identityKey []byte, testnet bool) (*Repo, error) {
	if sharedDB == nil {
		return nil, fmt.Errorf("sharedDB must not be nil")
	}
	if nodeID == "" {
		return nil, fmt.Errorf("nodeID must not be empty for shared DB mode")
	}

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create data directory %s: %w", dataDir, err)
	}

	pd := dbstore.NewDBPublicData(sharedDB, nodeID)
	db, err := dbstore.NewTenantDBWithPublicData(sharedDB, nodeID, pd)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant DB: %w", err)
	}

	// Schema migration for shared DB runs only once (DDL is global, not per-tenant).
	// Uses autoMigrateDatabaseManagedEscrow which avoids DROP TABLE — destructive DDL would
	// affect all tenants sharing the database.
	sharedDBMigrateOnce.Do(func() {
		sharedDBMigrateErr = autoMigrateDatabaseManagedEscrow(db)
	})
	if sharedDBMigrateErr != nil {
		return nil, fmt.Errorf("failed to auto-migrate shared DB: %w", sharedDBMigrateErr)
	}

	// Check if this tenant already has keys
	hasKeys := false
	if err := db.View(func(tx database.Tx) error {
		var key models.Key
		if err := tx.Read().Where("name = ?", "identity").First(&key).Error; err != nil {
			return err
		}
		hasKeys = true
		return nil
	}); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check tenant keys: %w", err)
	}

	if !hasKeys {
		keys, err := generateNodeKeys("", identityKey)
		if err != nil {
			return nil, fmt.Errorf("failed to generate node keys: %w", err)
		}
		if err := db.Update(func(tx database.Tx) error {
			if err := saveNodeKeys(tx, keys); err != nil {
				return err
			}
			return saveDefaultPreferences(tx)
		}); err != nil {
			return nil, fmt.Errorf("failed to save tenant keys: %w", err)
		}
	} else if len(identityKey) > 0 {
		// Existing tenant but identity key provided (e.g., from KeyVault) — update it
		if err := db.Update(func(tx database.Tx) error {
			return tx.Save(&models.Key{Name: "identity", Value: identityKey})
		}); err != nil {
			return nil, fmt.Errorf("failed to update identity key: %w", err)
		}
	}

	return &Repo{dataDir: dataDir, db: db}, nil
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

// IsRepoInitialized checks whether the data directory has been initialized
// by looking for the version file. This replaces the previous fsrepo.IsInitialized check.
func IsRepoInitialized(dataDir string) bool {
	_, err := os.Stat(path.Join(dataDir, versionFileName))
	return err == nil
}

func newRepo(nodeID string, dataDir, mnemonicSeed string, externalIdentityKey []byte, inMemoryDB bool, testnet bool) (*Repo, error) {
	var (
		keys  *nodeKeys
		err   error
		isNew bool
	)

	var torKey *models.Key

	if !IsRepoInitialized(dataDir) {
		if err := checkWriteable(dataDir); err != nil {
			return nil, err
		}

		keys, err = generateNodeKeys(mnemonicSeed, externalIdentityKey)
		if err != nil {
			return nil, err
		}

		_, torPriv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		torKey = &models.Key{Name: "tor", Value: torPriv.Seed()}

		isNew = true
	}

	var db database.Database
	if inMemoryDB {
		db, err = dbstore.NewMemoryDB(dataDir)
	} else {
		db, err = dbstore.NewSqliteDB(dataDir)
	}
	if err != nil {
		return nil, err
	}

	if err := autoMigrateDatabase(db); err != nil {
		return nil, err
	}

	needKeys := isNew
	if !isNew {
		var identityKey models.Key
		if dbErr := db.View(func(tx database.Tx) error {
			return tx.Read().Where("name = ?", "identity").First(&identityKey).Error
		}); dbErr != nil {
			needKeys = true
			keys, err = generateNodeKeys(mnemonicSeed, externalIdentityKey)
			if err != nil {
				return nil, err
			}
			_, torPriv, genErr := ed25519.GenerateKey(rand.Reader)
			if genErr != nil {
				return nil, genErr
			}
			torKey = &models.Key{Name: "tor", Value: torPriv.Seed()}
		}
	}

	if needKeys {
		err = db.Update(func(tx database.Tx) error {
			if err := saveNodeKeys(tx, keys); err != nil {
				return err
			}
			if err := tx.Save(torKey); err != nil {
				return err
			}
			return saveDefaultPreferences(tx)
		})
		if err != nil {
			return nil, err
		}
	}

	if err := CheckAndSetUlimit(); err != nil {
		return nil, err
	}

	r := &Repo{dataDir: dataDir, db: db}
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
		return fmt.Errorf("failed to save escrow key: %v", err)
	}

	if err := tx.Save(&models.Key{
		Name:  "ratings",
		Value: ratingKey.Serialize(),
	}); err != nil {
		return fmt.Errorf("failed to save rating key: %v", err)
	}

	if err := tx.Save(&models.Key{
		Name:  "bip44",
		Value: []byte(bip44Key.String()),
	}); err != nil {
		return fmt.Errorf("failed to save BIP44 key: %v", err)
	}

	if err := tx.Save(&models.Key{
		Name:  "solana",
		Value: []byte(*solKey),
	}); err != nil {
		return fmt.Errorf("failed to save SOL key: %v", err)
	}

	return nil
}

func GetKeysFromDB(tx database.Tx) (dbEscrowKey, dbBip44Key, dbSolKey, dbRatingKey models.Key, err error) {
	var escrowKey, ratingKey, bip44Key, solKey models.Key
	err = func() error {
		if err := tx.Read().Where("name = ?", "escrow").First(&escrowKey).Error; err != nil {
			return fmt.Errorf("failed to get escrow key: %v", err)
		}

		if err := tx.Read().Where("name = ?", "ratings").First(&ratingKey).Error; err != nil {
			return fmt.Errorf("failed to get rating key: %v", err)
		}

		if err := tx.Read().Where("name = ?", "bip44").First(&bip44Key).Error; err != nil {
			return fmt.Errorf("failed to get BIP44 key: %v", err)
		}

		if err := tx.Read().Where("name = ?", "solana").First(&solKey).Error; err != nil {
			return fmt.Errorf("failed to get SOL key: %v", err)
		}

		return nil
	}()
	if err != nil {
		return models.Key{}, models.Key{}, models.Key{}, models.Key{}, err
	}

	return escrowKey, bip44Key, solKey, ratingKey, nil
}

func (r *Repo) WriteUserAgent(comment string) error {
	uaPath := path.Join(r.dataDir, "user_agent")
	return os.WriteFile(uaPath, []byte(fmt.Sprintf("%s%s", version.UserAgent(), comment)), os.ModePerm)
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

// nodeKeys holds all cryptographic keys generated for a node.
type nodeKeys struct {
	identityKey  []byte
	escrowKey    *btcec.PrivateKey
	ratingKey    *btcec.PrivateKey
	bip44Key     *hdkeychain.ExtendedKey
	solKey       *ed25519.PrivateKey
	mnemonicSeed string
}

// generateNodeKeys creates all node cryptographic keys.
// If mnemonicSeed is empty, a new one is generated.
// If externalIdentityKey is provided, it overrides the mnemonic-derived identity key.
func generateNodeKeys(mnemonicSeed string, externalIdentityKey []byte) (*nodeKeys, error) {
	var err error
	if mnemonicSeed == "" {
		mnemonicSeed, err = createMnemonic(bip39.NewEntropy, bip39.NewMnemonic)
		if err != nil {
			return nil, err
		}
	}

	var identityKey []byte
	if len(externalIdentityKey) > 0 {
		identityKey = externalIdentityKey
	} else {
		identitySeed := bip39.NewSeed(mnemonicSeed, "Secret Passphrase")
		identityKey, err = IdentityKeyFromSeed(identitySeed, 0)
		if err != nil {
			return nil, err
		}
	}

	hdSeed := bip39.NewSeed(mnemonicSeed, "")
	escrowKey, ratingKey, bip44Key, solKey, err := CreateHDKeys(hdSeed)
	if err != nil {
		return nil, err
	}

	return &nodeKeys{
		identityKey:  identityKey,
		escrowKey:    escrowKey,
		ratingKey:    ratingKey,
		bip44Key:     bip44Key,
		solKey:       solKey,
		mnemonicSeed: mnemonicSeed,
	}, nil
}

// saveNodeKeys persists all node keys (identity, wallet, mnemonic) to the database.
func saveNodeKeys(tx database.Tx, keys *nodeKeys) error {
	for _, k := range []*models.Key{
		{Name: "identity", Value: keys.identityKey},
		{Name: "escrow", Value: keys.escrowKey.Serialize()},
		{Name: "ratings", Value: keys.ratingKey.Serialize()},
		{Name: "bip44", Value: []byte(keys.bip44Key.String())},
		{Name: "solana", Value: []byte(*keys.solKey)},
		{Name: "mnemonic", Value: []byte(keys.mnemonicSeed)},
	} {
		if err := tx.Save(k); err != nil {
			return err
		}
	}
	return nil
}

// saveDefaultPreferences saves the default user preferences for a new node.
func saveDefaultPreferences(tx database.Tx) error {
	return tx.Save(&models.UserPreferences{
		AutoConfirm:       true,
		MisPaymentBuffer:  defaultMispaymentBuffer,
		ShowNsfw:          true,
		ShowNotifications: true,
	})
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

func autoMigrateDatabase(db database.Database) error {
	dbModels := []interface{}{
		&models.Key{},
		&models.OutgoingMessage{},
		&models.IncomingMessage{},
		&models.NotificationRecord{},
		&models.FollowerStat{},
		&models.FollowSequence{},
		&models.Event{},
		&models.Order{},
		&models.TransactionMetadata{},
		&models.UserPreferences{},
		&models.StoreAndForwardServers{},
		&models.Case{},
		&models.StoreCartRecord{},
		&models.MatrixCredentials{},
		&models.Discount{},
		&models.DiscountCode{},
		&models.DiscountRedemption{},
		&models.ShippingProfileEntity{},
		&models.ShippingLocationEntity{},
		&models.ListingShippingRef{},
		&models.OutboxEvent{},
		&dbstore.PublicDataRecord{},
		&dbstore.PublicMediaRecord{},
		&models.GuestCheckoutConfig{},
		&models.GuestOrder{},
		&models.GuestOrderItem{},
		&models.InventoryReservation{},
		&models.DirectPaymentAddressCounter{},
		&models.SweepTask{},
		&models.DigitalAsset{},
		&models.DigitalLicenseKey{},
		&models.LicenseActivation{},
		&models.DownloadGrant{},
		&models.DigitalDownloadLog{},
		// Phase EVM-ManagedEscrow v0.3.0 — append-only fact table for the
		// monitor-driven payment model. See
		// docs/escrow/MONITOR_DRIVEN_PAYMENT.md (v2.0).
		&models.PaymentObservation{},
	}

	return db.Update(func(tx database.Tx) error {
		// Phase 1: Detect v4 tables that need composite PK rebuild.
		// GORM AutoMigrate cannot change primary keys in SQLite, so we
		// rename old tables, let AutoMigrate create new ones with the
		// correct (tenant_id, ...) composite PK, then copy data back.
		backups, err := prepareV4PKMigration(tx)
		if err != nil {
			return fmt.Errorf("v4 PK migration prep failed: %v", err)
		}

		// Phase 2: AutoMigrate all models
		for _, m := range dbModels {
			if err := tx.Migrate(m); err != nil {
				return fmt.Errorf("failed to migrate table %s: %v", reflect.TypeOf(m).String(), err)
			}
		}

		// Phase 3: Restore data from v4 backup tables
		if err := completeV4PKMigration(tx, backups); err != nil {
			return fmt.Errorf("v4 PK migration restore failed: %v", err)
		}

		// 特殊处理 ReceivingAccount 表
		// 检查表是否存在
		var count int64
		if err := tx.Read().Table("receiving_accounts").Count(&count).Error; err != nil {
			// 表不存在，直接创建
			if err := tx.Migrate(&models.ReceivingAccount{}); err != nil {
				return fmt.Errorf("failed to migrate ReceivingAccount: %v", err)
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
				return fmt.Errorf("failed to get table info: %v", err)
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
					return fmt.Errorf("failed to drop old receiving_accounts: %v", err)
				}

				// 创建新表
				if err := tx.Migrate(&models.ReceivingAccount{}); err != nil {
					return fmt.Errorf("failed to create receiving_accounts: %v", err)
				}
			}
		}

		return nil
	})
}

// v4BackupTable holds info needed to restore data after PK migration.
type v4BackupTable struct {
	tableName  string
	backupName string
	columns    []string // non-tenant_id column names from old table
}

// prepareV4PKMigration detects tables from old v4 databases that have
// single-column primary keys instead of the required composite PKs with
// tenant_id. For each such table it renames the old table to a backup so
// that AutoMigrate can create the correct table. Returns the list of tables
// whose data needs to be restored after AutoMigrate.
func prepareV4PKMigration(tx database.Tx) ([]v4BackupTable, error) {
	type pragmaCol struct {
		Name string `gorm:"column:name"`
		PK   int    `gorm:"column:pk"`
	}

	// Tables that had single-column PKs in v4 and now require composite
	// PKs including tenant_id. Tables added in v3.0.0 (discounts, shipping,
	// public_data, etc.) are excluded — they don't exist in v4.
	candidates := []string{
		"keys",
		"outgoing_messages",
		"incoming_messages",
		"notification_records",
		"follower_stats",
		"follow_sequences",
		"events",
		"orders",
		"transaction_metadata",
		"user_preferences",
		"store_and_forward_servers",
		"cases",
		"store_cart_records",
	}

	var backups []v4BackupTable

	for _, name := range candidates {
		var cols []pragmaCol
		tx.Read().Raw(fmt.Sprintf("PRAGMA table_info(`%s`)", name)).Scan(&cols)
		if len(cols) == 0 {
			continue
		}

		tenantInPK := false
		var dataCols []string
		for _, c := range cols {
			if c.Name == "tenant_id" {
				if c.PK > 0 {
					tenantInPK = true
				}
				continue
			}
			dataCols = append(dataCols, c.Name)
		}

		if tenantInPK {
			continue
		}

		backup := "_v4_bak_" + name

		// Drop any leftover from a previous failed attempt
		tx.Read().Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", backup))

		if err := tx.Read().Exec(
			fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", name, backup),
		).Error; err != nil {
			return nil, fmt.Errorf("rename %s → %s: %v", name, backup, err)
		}
		log.Infof("V4 PK migration: renamed %s → %s", name, backup)

		backups = append(backups, v4BackupTable{
			tableName:  name,
			backupName: backup,
			columns:    dataCols,
		})
	}

	return backups, nil
}

// completeV4PKMigration copies data from backup tables into the newly
// created tables (with correct composite PKs) and drops the backups.
func completeV4PKMigration(tx database.Tx, backups []v4BackupTable) error {
	for _, b := range backups {
		quoted := make([]string, len(b.columns))
		for i, c := range b.columns {
			quoted[i] = "`" + c + "`"
		}
		colList := strings.Join(quoted, ", ")

		sql := fmt.Sprintf(
			"INSERT OR IGNORE INTO `%s` (`tenant_id`, %s) SELECT '%s', %s FROM `%s`",
			b.tableName, colList,
			database.StandaloneTenantID,
			colList, b.backupName,
		)
		if err := tx.Read().Exec(sql).Error; err != nil {
			return fmt.Errorf("restore %s: %v", b.tableName, err)
		}

		if err := tx.Read().Exec(
			fmt.Sprintf("DROP TABLE `%s`", b.backupName),
		).Error; err != nil {
			return fmt.Errorf("drop backup %s: %v", b.backupName, err)
		}

		log.Infof("V4 PK migration: restored %s (%d cols)", b.tableName, len(b.columns))
	}
	return nil
}

// autoMigrateDatabaseManagedEscrow is the shared-DB variant of autoMigrateDatabase.
// It creates all tables using AutoMigrate only — no DROP TABLE, no PRAGMA table_info.
// This is safe for multi-tenant shared databases where destructive DDL would affect
// all tenants. The shared DB is always freshly created by initSharedNodeDB, so legacy
// schema migration is unnecessary.
func autoMigrateDatabaseManagedEscrow(db database.Database) error {
	allModels := []interface{}{
		&models.Key{},
		&models.OutgoingMessage{},
		&models.IncomingMessage{},
		&models.NotificationRecord{},
		&models.FollowerStat{},
		&models.FollowSequence{},
		&models.Event{},
		&models.Order{},
		&models.TransactionMetadata{},
		&models.UserPreferences{},
		&models.StoreAndForwardServers{},
		&models.Case{},
		&models.StoreCartRecord{},
		&models.MatrixCredentials{},
		&models.ReceivingAccount{},
		&models.Discount{},
		&models.DiscountCode{},
		&models.DiscountRedemption{},
		&models.ShippingProfileEntity{},
		&models.ShippingLocationEntity{},
		&models.ListingShippingRef{},
		&models.OutboxEvent{},
		&models.GuestCheckoutConfig{},
		&models.GuestOrder{},
		&models.GuestOrderItem{},
		&models.InventoryReservation{},
		&models.DirectPaymentAddressCounter{},
		&models.SweepTask{},
		&models.DigitalAsset{},
		&models.DigitalLicenseKey{},
		&models.LicenseActivation{},
		&models.DownloadGrant{},
		&models.DigitalDownloadLog{},
		// Phase EVM-ManagedEscrow v0.3.0 — append-only fact table for the
		// monitor-driven payment model. See
		// docs/escrow/MONITOR_DRIVEN_PAYMENT.md (v2.0).
		&models.PaymentObservation{},
	}

	return db.Update(func(tx database.Tx) error {
		for _, m := range allModels {
			if err := tx.Migrate(m); err != nil {
				return fmt.Errorf("migrate %s failed: %v", reflect.TypeOf(m).String(), err)
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

	if version < 5 {
		log.Infof("Migrating repo from version %d to %d", version, 5)
		if err := migrateRepoToNodesStructure(dataDir); err != nil {
			return fmt.Errorf("migration failed: %v", err)
		}
		// Write updated version at root so migration won't re-trigger.
		// (The old version file was moved into nodes/default/ by the migration.)
		versionStr := strconv.Itoa(DefaultRepoVersion)
		if err := os.WriteFile(path.Join(dataDir, versionFileName), []byte(versionStr), os.ModePerm); err != nil {
			return fmt.Errorf("failed to write root version after migration: %v", err)
		}
	}

	return nil
}

func migrateRepoToNodesStructure(rootPath string) error {
	defaultNodePath := path.Join(rootPath, "nodes", DefaultNodeID)

	if err := os.MkdirAll(defaultNodePath, 0755); err != nil {
		return fmt.Errorf("failed to create nodes directory structure: %v", err)
	}

	itemsToMove := []string{
		common.PublicDirName,
		common.DatabaseFileName,
		versionFileName,
	}

	for _, item := range itemsToMove {
		srcPath := path.Join(rootPath, item)
		dstPath := path.Join(defaultNodePath, item)

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
