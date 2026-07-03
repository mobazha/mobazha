package dbstore

import (
	"fmt"
	"os"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha/pkg/posts/pb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type deleteListing string

type deletePost string

type encryptedListing struct {
	slug string
	data []byte
}

type mediaCIDIndex struct {
	cidHash     string
	mediaType   string
	sizeTag     string
	name        string
	contentType string
}

// TenantDB is a multi-tenant Database implementation that wraps a shared
// *gorm.DB and automatically scopes all operations to a specific tenant.
//
// In SaaS/hosting mode, multiple tenants share a single database. TenantDB
// ensures data isolation by:
//   - Injecting WHERE tenant_id = ? on all Read() queries
//   - Auto-setting TenantID on all Save() operations
//   - Scoping Update()/Delete()/DeleteAll() to the tenant
//
// Public data storage uses DBPublicData (stores data in relational DB)
// for both standalone and SaaS modes.
type TenantDB struct {
	sharedDB   *gorm.DB
	tenantID   string
	publicData database.PublicData
	mtx        sync.Mutex
}

// TenantID returns the tenant scope carried by this Database wrapper.
// Core runtime wiring uses it when a cross-cutting component needs both
// the tenant-scoped database interface and the stable tenant identifier.
func (tdb *TenantDB) TenantID() string {
	return tdb.tenantID
}

// RawDB returns the underlying shared *gorm.DB. This is intentionally
// narrow and only used by subsystems that must perform cross-tenant or
// chain-global sweeps while still living alongside the tenant-scoped DB
// abstraction.
func (tdb *TenantDB) RawDB() *gorm.DB {
	return tdb.sharedDB
}

// ForTenant returns a new tenant-scoped Database view over the same shared
// database. Cross-cutting sinks use this only when an event explicitly names
// a different target tenant than the node currently dispatching it.
func (tdb *TenantDB) ForTenant(tenantID string) (database.Database, error) {
	return NewTenantDBWithPublicData(tdb.sharedDB, tenantID, NewDBPublicData(tdb.sharedDB, tenantID))
}

// NewTenantDBWithPublicData creates a new TenantDB with a given PublicData
// implementation. Both standalone (DBPublicData on local SQLite) and SaaS
// (DBPublicData on shared PostgreSQL) use this constructor.
func NewTenantDBWithPublicData(sharedDB *gorm.DB, tenantID string, pd database.PublicData) (database.Database, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantDB: tenantID must not be empty")
	}
	return &TenantDB{
		sharedDB:   sharedDB,
		tenantID:   tenantID,
		publicData: pd,
		mtx:        sync.Mutex{},
	}, nil
}

// View invokes the passed function in the context of a managed
// read-only transaction with tenant scoping.
func (tdb *TenantDB) View(fn func(tx database.Tx) error) error {
	tdb.mtx.Lock()
	defer tdb.mtx.Unlock()

	tx := tdb.readTx()
	err := tdb.safeTxExec(tx, fn)
	return err
}

// Update invokes the passed function in the context of a managed
// read-write transaction with tenant scoping.
func (tdb *TenantDB) Update(fn func(tx database.Tx) error) error {
	tdb.mtx.Lock()
	defer tdb.mtx.Unlock()

	tx := tdb.writeTx()
	err := tdb.safeTxExec(tx, fn)
	return err
}

// safeTxExec executes fn within the transaction, recovering from panics
// to ensure the transaction is always rolled back and connections are not leaked.
func (tdb *TenantDB) safeTxExec(tx *tenantTx, fn func(tx database.Tx) error) (retErr error) {
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			retErr = fmt.Errorf("panic in tenant transaction: %v\n%s", p, debug.Stack())
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ComputePublicDataHash returns a CID-compatible hash of all structured
// public data records for this tenant, used for publish change detection.
func (tdb *TenantDB) ComputePublicDataHash() (cid.Cid, error) {
	tdb.mtx.Lock()
	defer tdb.mtx.Unlock()

	if dbpd, ok := tdb.publicData.(*DBPublicData); ok {
		return dbpd.ComputeContentHash()
	}
	return cid.Undef, fmt.Errorf("publicData backend does not support ComputeContentHash")
}

// Close cleanly shuts down the tenant DB view.
// Note: does NOT close the shared *gorm.DB (owned by the host).
func (tdb *TenantDB) Close() error {
	return nil
}

func (tdb *TenantDB) readTx() *tenantTx {
	return &tenantTx{
		baseDB:      tdb.sharedDB,
		publicData:  tdb.publicData,
		tenantID:    tdb.tenantID,
		isForWrites: false,
	}
}

func (tdb *TenantDB) writeTx() *tenantTx {
	dbtx := tdb.sharedDB.Begin()
	return &tenantTx{
		baseDB:      dbtx,
		rawTx:       dbtx,
		publicData:  tdb.publicData,
		tenantID:    tdb.tenantID,
		isForWrites: true,
	}
}

// tenantTx is a tenant-scoped transaction that automatically filters
// and injects tenant_id for all database operations.
//
// IMPORTANT: Read() creates a fresh GORM session each time it is called.
// This prevents Statement/Schema state from leaking between queries on
// different models within the same View/Update callback — a subtle GORM bug
// where First(&modelA) followed by First(&modelB) on the same *gorm.DB
// instance can cause the second query to use modelA's schema/table.
type tenantTx struct {
	baseDB     *gorm.DB // base for creating fresh Read() sessions (sharedDB for reads, rawTx for writes)
	rawTx      *gorm.DB // un-scoped tx for Commit/Rollback (write mode only)
	publicData database.PublicData

	tenantID string

	rollbackCache []interface{}
	commitCache   []interface{}
	commitHooks   []func()

	closed      bool
	isForWrites bool
}

// Commit commits all changes.
func (t *tenantTx) Commit() error {
	if t.closed {
		panic("tx already closed")
	}
	defer func() { t.closed = true }()

	if !t.isForWrites {
		return nil
	}

	// Route public data writes through the active GORM transaction to avoid
	// SQLite lock contention (the shared DB and the transaction share the
	// same underlying connection in SQLite).
	if dbpd, ok := t.publicData.(*DBPublicData); ok {
		t.publicData = dbpd.WithDB(t.baseDB)
	}

	for _, i := range t.commitCache {
		if err := t.setInterfaceType(i); err != nil {
			t.Rollback()
			return err
		}
	}

	// Use rawTx for commit (not the scoped session)
	if err := t.rawTx.Commit().Error; err != nil {
		t.Rollback()
		return err
	}
	for _, fn := range t.commitHooks {
		fn()
	}
	return nil
}

// Rollback undoes all changes.
func (t *tenantTx) Rollback() error {
	if t.closed {
		panic("tx already closed")
	}
	defer func() { t.closed = true }()

	if !t.isForWrites {
		return nil
	}

	// Route rollback writes through the active GORM transaction (same
	// rationale as Commit — avoid SQLite lock contention). With DBPublicData,
	// these writes are effectively no-ops since rawTx.Rollback() below
	// undoes them, but they must not deadlock.
	if dbpd, ok := t.publicData.(*DBPublicData); ok {
		t.publicData = dbpd.WithDB(t.baseDB)
	}

	for _, i := range t.rollbackCache {
		if err := t.setInterfaceType(i); err != nil {
			return err
		}
	}

	if err := t.rawTx.Rollback().Error; err != nil {
		return err
	}
	return nil
}

// Read returns a tenant-scoped *gorm.DB for queries.
// Each call creates a fresh GORM session with WHERE tenant_id = ? applied,
// preventing Statement/Schema leakage between queries on different models.
func (t *tenantTx) Read() *gorm.DB {
	return t.baseDB.Session(&gorm.Session{NewDB: true}).Where("tenant_id = ?", t.tenantID)
}

// Save saves the model, auto-setting TenantID before saving.
// Uses rawTx (not the scoped dbtx) because GORM's Save() on a WHERE-scoped
// session incorrectly treats new inserts as updates when existing records
// match the scope conditions. TenantID is set on the model itself.
// For models with composite PKs (e.g. tenant_id + id), uses compositePKSave
// to generate correct ON CONFLICT clauses for SQLite.
func (t *tenantTx) Save(model interface{}) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	setTenantID(model, t.tenantID)
	return compositePKSave(t.rawTx, model)
}

// Create inserts a model once, with the same tenant injection guarantee as
// Save. A uniqueness conflict is returned to the caller and is never changed
// into an update.
func (t *tenantTx) Create(model interface{}) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	setTenantID(model, t.tenantID)
	return t.rawTx.Create(model).Error
}

// Update updates the given column with tenant scoping.
// Uses rawTx with explicit tenant WHERE to avoid session condition accumulation.
func (t *tenantTx) Update(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	db := t.rawTx.Where("tenant_id = ?", t.tenantID).Model(model)
	for k, v := range where {
		db = tenantWhere(db, k, v)
	}
	return db.UpdateColumn(key, value).Error
}

// UpdateColumns updates multiple columns with tenant scoping and returns the
// affected row count. Use this for compare-and-swap style writes that need
// RowsAffected without dropping down to tx.Read().Updates().
func (t *tenantTx) UpdateColumns(values map[string]interface{}, where map[string]interface{}, model interface{}) (int64, error) {
	if !t.isForWrites {
		return 0, ErrReadOnly
	}
	db := t.rawTx.Where("tenant_id = ?", t.tenantID).Model(model)
	for k, v := range where {
		db = tenantWhere(db, k, v)
	}
	res := db.UpdateColumns(values)
	return res.RowsAffected, res.Error
}

// tenantWhere keeps the Tx condition-map API portable across SQL dialects.
// GORM binds `nil` in a raw `IS ?` predicate as a normal parameter. SQLite
// accepts `IS ?`, but PostgreSQL renders it as `IS $n`, which is invalid SQL;
// PostgreSQL requires the NULL keyword to be part of the statement instead.
func tenantWhere(db *gorm.DB, query string, value interface{}) *gorm.DB {
	if value == nil {
		trimmed := strings.TrimSpace(query)
		upper := strings.ToUpper(trimmed)
		if strings.HasSuffix(upper, " IS ?") || strings.HasSuffix(upper, " IS NOT ?") {
			return db.Where(strings.TrimSuffix(trimmed, "?") + "NULL")
		}
	}
	return db.Where(query, value)
}

// Delete deletes records with tenant scoping.
// Uses rawTx with explicit tenant WHERE to avoid session condition accumulation.
func (t *tenantTx) Delete(key string, value interface{}, where map[string]interface{}, model interface{}) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	db := t.rawTx.Where("tenant_id = ?", t.tenantID).Model(model)
	for k, v := range where {
		db = tenantWhere(db, k, v)
	}
	if strings.Contains(key, "?") {
		return db.Where(key, value).Delete(model).Error
	}
	return db.Where(fmt.Sprintf("%s = ?", key), value).Delete(model).Error
}

// DeleteAll deletes all records of the given model type for this tenant.
// Uses rawTx with explicit tenant WHERE to avoid session condition accumulation.
func (t *tenantTx) DeleteAll(model interface{}) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	return t.rawTx.Where("tenant_id = ?", t.tenantID).Where("1 = 1").Delete(model).Error
}

// Migrate auto-migrates the model schema.
func (t *tenantTx) Migrate(model interface{}) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	// Migration uses raw tx (schema changes are global, not tenant-scoped)
	if t.rawTx != nil {
		return t.rawTx.AutoMigrate(model)
	}
	return t.baseDB.AutoMigrate(model)
}

// RegisterCommitHook registers a callback invoked on successful commit.
func (t *tenantTx) RegisterCommitHook(fn func()) {
	t.commitHooks = append(t.commitHooks, fn)
}

// ========== PublicData methods (flat file, same as FFSqliteDB) ==========

func (t *tenantTx) GetProfile() (*models.Profile, error) {
	for x := len(t.commitCache) - 1; x >= 0; x-- {
		profile, ok := t.commitCache[x].(*models.Profile)
		if ok {
			return profile, nil
		}
	}
	return t.publicData.GetProfile()
}

func (t *tenantTx) SetProfile(profile *models.Profile) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	current, err := t.publicData.GetProfile()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	t.rollbackCache = append(t.rollbackCache, current)
	t.commitCache = append(t.commitCache, profile)
	return nil
}

func (t *tenantTx) GetFollowers() (models.Followers, error) {
	for x := len(t.commitCache) - 1; x >= 0; x-- {
		followers, ok := t.commitCache[x].(models.Followers)
		if ok {
			return followers, nil
		}
	}
	return t.publicData.GetFollowers()
}

func (t *tenantTx) SetFollowers(followers models.Followers) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	current, err := t.publicData.GetFollowers()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	t.rollbackCache = append(t.rollbackCache, current)
	t.commitCache = append(t.commitCache, followers)
	return nil
}

func (t *tenantTx) GetFollowing() (models.Following, error) {
	for x := len(t.commitCache) - 1; x >= 0; x-- {
		following, ok := t.commitCache[x].(models.Following)
		if ok {
			return following, nil
		}
	}
	return t.publicData.GetFollowing()
}

func (t *tenantTx) SetFollowing(following models.Following) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	current, err := t.publicData.GetFollowing()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	t.rollbackCache = append(t.rollbackCache, current)
	t.commitCache = append(t.commitCache, following)
	return nil
}

func (t *tenantTx) GetListing(slug string) (*pb.SignedListing, error) {
	for x := len(t.commitCache) - 1; x >= 0; x-- {
		listing, ok := t.commitCache[x].(*pb.SignedListing)
		if ok && listing.Listing.Slug == slug {
			return listing, nil
		}
	}
	return t.publicData.GetListing(slug)
}

func (t *tenantTx) SetListing(listing *pb.SignedListing) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	current, err := t.publicData.GetListing(listing.Listing.Slug)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	t.rollbackCache = append(t.rollbackCache, current)
	t.commitCache = append(t.commitCache, listing)
	return nil
}

func (t *tenantTx) GetEncryptedListing(slug string) ([]byte, error) {
	for x := len(t.commitCache) - 1; x >= 0; x-- {
		enc, ok := t.commitCache[x].(encryptedListing)
		if ok && enc.slug == slug {
			return enc.data, nil
		}
	}
	return t.publicData.GetEncryptedListing(slug)
}

func (t *tenantTx) SetEncryptedListing(slug string, encryptedData []byte) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	current, err := t.publicData.GetEncryptedListing(slug)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	t.rollbackCache = append(t.rollbackCache, encryptedListing{slug: slug, data: current})
	t.commitCache = append(t.commitCache, encryptedListing{slug: slug, data: encryptedData})
	return nil
}

func (t *tenantTx) DeleteListing(slug string) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	current, err := t.publicData.GetListing(slug)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	t.rollbackCache = append(t.rollbackCache, current)
	t.commitCache = append(t.commitCache, deleteListing(slug))
	return nil
}

func (t *tenantTx) GetListingIndex() (models.ListingIndex, error) {
	for x := len(t.commitCache) - 1; x >= 0; x-- {
		index, ok := t.commitCache[x].(models.ListingIndex)
		if ok {
			return index, nil
		}
	}
	return t.publicData.GetListingIndex()
}

func (t *tenantTx) SetListingIndex(index models.ListingIndex) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	current, err := t.publicData.GetListingIndex()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	t.rollbackCache = append(t.rollbackCache, current)
	t.commitCache = append(t.commitCache, index)
	return nil
}

func (t *tenantTx) GetRatingIndex() (models.RatingIndex, error) {
	for x := len(t.commitCache) - 1; x >= 0; x-- {
		index, ok := t.commitCache[x].(models.RatingIndex)
		if ok {
			return index, nil
		}
	}
	return t.publicData.GetRatingIndex()
}

func (t *tenantTx) SetRatingIndex(index models.RatingIndex) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	current, err := t.publicData.GetRatingIndex()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	t.rollbackCache = append(t.rollbackCache, current)
	t.commitCache = append(t.commitCache, index)
	return nil
}

func (t *tenantTx) SetRating(rating *pb.Rating) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	t.commitCache = append(t.commitCache, rating)
	return nil
}

func (t *tenantTx) GetPostIndex() ([]models.PostData, error) {
	for x := len(t.commitCache) - 1; x >= 0; x-- {
		index, ok := t.commitCache[x].([]models.PostData)
		if ok {
			return index, nil
		}
	}
	return t.publicData.GetPostIndex()
}

func (t *tenantTx) SetPostIndex(index []models.PostData) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	current, err := t.publicData.GetPostIndex()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	t.rollbackCache = append(t.rollbackCache, current)
	t.commitCache = append(t.commitCache, index)
	return nil
}

func (t *tenantTx) AddPost(post *postsPb.SignedPost) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	t.commitCache = append(t.commitCache, post)
	return nil
}

func (t *tenantTx) DeletePost(slug string) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	current, err := t.publicData.GetPost(slug)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	t.rollbackCache = append(t.rollbackCache, current)
	t.commitCache = append(t.commitCache, deletePost(slug))
	return nil
}

func (t *tenantTx) PostExist(slug string) bool {
	return t.publicData.PostExist(slug)
}

func (t *tenantTx) GetPost(slug string) (*postsPb.SignedPost, error) {
	for x := len(t.commitCache) - 1; x >= 0; x-- {
		post, ok := t.commitCache[x].(*postsPb.SignedPost)
		if ok && post.Post.Slug == slug {
			return post, nil
		}
	}
	return t.publicData.GetPost(slug)
}

func (t *tenantTx) SetImage(img models.Image) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	t.commitCache = append(t.commitCache, img)
	return nil
}

func (t *tenantTx) GetImageByName(size models.ImageSize, name string) ([]byte, error) {
	return t.publicData.GetImageByName(size, name)
}

func (t *tenantTx) GetMediaByCID(cidHash string) ([]byte, string, error) {
	return t.publicData.GetMediaByCID(cidHash)
}

func (t *tenantTx) IndexMediaCID(cidHash string, mediaType string, sizeTag string, name string, contentType string) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	t.commitCache = append(t.commitCache, mediaCIDIndex{
		cidHash: cidHash, mediaType: mediaType, sizeTag: sizeTag,
		name: name, contentType: contentType,
	})
	return nil
}

func (t *tenantTx) SetUploadedFile(file models.UploadedFile) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	t.commitCache = append(t.commitCache, file)
	return nil
}

func (t *tenantTx) SetIntroVideo(introVideo models.IntroVideo) error {
	if !t.isForWrites {
		return ErrReadOnly
	}
	t.commitCache = append(t.commitCache, introVideo)
	return nil
}

// setInterfaceType commits cached public data changes via the PublicData interface.
func (t *tenantTx) setInterfaceType(i interface{}) error {
	switch i := i.(type) {
	case *models.Profile:
		if i == nil {
			return nil
		}
		return t.publicData.SetProfile(i)
	case models.Followers:
		return t.publicData.SetFollowers(i)
	case models.Following:
		return t.publicData.SetFollowing(i)
	case *pb.SignedListing:
		if i == nil {
			return nil
		}
		return t.publicData.SetListing(i)
	case models.ListingIndex:
		return t.publicData.SetListingIndex(i)
	case models.RatingIndex:
		return t.publicData.SetRatingIndex(i)
	case *pb.Rating:
		if i == nil {
			return nil
		}
		return t.publicData.SetRating(i)
	case []models.PostData:
		return t.publicData.SetPostIndex(i)
	case *postsPb.SignedPost:
		if i == nil {
			return nil
		}
		return t.publicData.AddPost(i)
	case models.Image:
		return t.publicData.SetImage(i)
	case models.UploadedFile:
		return t.publicData.SetUploadedFile(i)
	case mediaCIDIndex:
		return t.publicData.IndexMediaCID(i.cidHash, i.mediaType, i.sizeTag, i.name, i.contentType)
	case models.IntroVideo:
		return t.publicData.SetIntroVideo(i)
	case encryptedListing:
		return t.publicData.SetEncryptedListing(i.slug, i.data)
	case deleteListing:
		return t.publicData.DeleteListing(string(i))
	case deletePost:
		return t.publicData.DeletePost(string(i))
	}
	return nil
}

// compositePKSave performs an upsert that works correctly with composite
// primary keys. GORM's Save() generates ON CONFLICT(id) which fails when the
// table has a composite PK like (tenant_id, id). This function uses
// Create + OnConflict with all PK columns explicitly listed.
//
// For new records (integer PK field == 0), it assigns the next available ID
// by querying MAX(id)+1, since auto-increment columns that are part of a
// composite primary key require manual ID assignment.
func compositePKSave(db *gorm.DB, model interface{}) error {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return db.Save(model).Error
	}

	var pkColumns []clause.Column
	for _, field := range stmt.Schema.PrimaryFields {
		pkColumns = append(pkColumns, clause.Column{Name: field.DBName})
	}
	if len(pkColumns) < 2 {
		return db.Save(model).Error
	}

	assignAutoID(db, stmt, model)

	return db.Clauses(clause.OnConflict{
		Columns:   pkColumns,
		UpdateAll: true,
	}).Create(model).Error
}

// assignAutoID checks for integer PK fields with zero value and assigns
// MAX(column)+1 within the current GORM scope (which includes tenant_id
// filtering). This replaces SQLite's AUTOINCREMENT for composite PKs.
func assignAutoID(db *gorm.DB, stmt *gorm.Statement, model interface{}) {
	v := reflect.ValueOf(model)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	for _, field := range stmt.Schema.PrimaryFields {
		if field.DBName == "tenant_id" {
			continue
		}
		fv := v.FieldByName(field.Name)
		if !fv.IsValid() || fv.Kind() != reflect.Int {
			continue
		}
		if fv.Int() != 0 {
			continue
		}
		var maxID int
		db.Model(model).Select(fmt.Sprintf("COALESCE(MAX(%s), 0)", field.DBName)).Scan(&maxID)
		fv.SetInt(int64(maxID + 1))
	}
}

// setTenantID uses reflection to set the TenantID field on a model if it has
// an embedded TenantMixin. This is called automatically by Save().
func setTenantID(model interface{}, tenantID string) {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	// Strategy 1: Look for TenantMixin embedded struct (most models)
	mixin := v.FieldByName("TenantMixin")
	if mixin.IsValid() && mixin.CanSet() {
		tidField := mixin.FieldByName("TenantID")
		if tidField.IsValid() && tidField.CanSet() && tidField.Kind() == reflect.String {
			tidField.SetString(tenantID)
			return
		}
	}

	// Strategy 2: Look for direct TenantID field (models with composite PK like Key, StoreCartRecord)
	tidField := v.FieldByName("TenantID")
	if tidField.IsValid() && tidField.CanSet() && tidField.Kind() == reflect.String {
		tidField.SetString(tenantID)
	}
}
