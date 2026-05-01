package scheduler

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SchedulerLock is the GORM model backing scheduler_locks. One row per Job.
//
// The schema is dialect-agnostic: GORM's AutoMigrate handles SQLite (standalone
// developer DBs) and PostgreSQL (SaaS production). The acquire / renew / release
// helpers use only portable GORM operations (Updates / Create / Delete) so no
// dialect-specific clauses (FOR UPDATE NOWAIT, ON CONFLICT) are needed.
type SchedulerLock struct {
	JobName    string    `gorm:"column:job_name;primaryKey;size:128"`
	HolderID   string    `gorm:"column:holder_id;size:128;not null"`
	AcquiredAt time.Time `gorm:"column:acquired_at;not null"`
	ExpiresAt  time.Time `gorm:"column:expires_at;index;not null"`
}

// TableName is the explicit table name for SchedulerLock.
func (SchedulerLock) TableName() string { return "scheduler_locks" }

// ErrLockLost is returned by renew when the holder no longer owns the lock.
var ErrLockLost = errors.New("scheduler: lock lost")

// MigrateLocks creates or upgrades the scheduler_locks table.
//
// Callers should run this once at process start before Scheduler.Start().
// ManagedEscrow to call repeatedly; AutoMigrate is idempotent.
func MigrateLocks(db *gorm.DB) error {
	return db.AutoMigrate(&SchedulerLock{})
}

// tryAcquire attempts to take the lock for jobName with the given TTL.
//
// Acquisition succeeds when either:
//  1. No row exists for jobName (Create); or
//  2. The existing row has expired (UPDATE WHERE expires_at < now); or
//  3. The existing row already belongs to holderID (re-acquire / renew).
//
// Returns false when another live holder owns the lock or the create races.
// Errors from the underlying DB driver are returned as-is.
func tryAcquire(db *gorm.DB, jobName, holderID string, ttl time.Duration, now func() time.Time) (bool, error) {
	t := now().UTC()
	expires := t.Add(ttl)

	res := db.Model(&SchedulerLock{}).
		Where("job_name = ? AND (expires_at < ? OR holder_id = ?)", jobName, t, holderID).
		Updates(map[string]interface{}{
			"holder_id":   holderID,
			"acquired_at": t,
			"expires_at":  expires,
		})
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected > 0 {
		return true, nil
	}

	// No existing row to update — try INSERT. A unique-violation here means
	// a concurrent caller created the row first; treat as "lost the race".
	// Any other error (table missing, connection lost, permission denied) is
	// a real failure and must be surfaced to the caller.
	createErr := db.Create(&SchedulerLock{
		JobName:    jobName,
		HolderID:   holderID,
		AcquiredAt: t,
		ExpiresAt:  expires,
	}).Error
	if createErr != nil {
		if isDuplicateKeyErr(createErr) {
			return false, nil
		}
		return false, createErr
	}
	return true, nil
}

// isDuplicateKeyErr returns true when the error is a unique-constraint /
// duplicate-key violation from any supported dialect (SQLite or PostgreSQL).
func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique_violation") ||
		strings.Contains(msg, "constraint failed")
}

// renew extends the lease for jobName. Returns ErrLockLost if holderID no
// longer owns the row (e.g. another instance acquired it after expiry).
//
// Currently unused by AH-3a Foundation (Jobs run shorter than LeaseTTL), but
// exposed for future long-running global Jobs introduced in AH-3b.
func renew(db *gorm.DB, jobName, holderID string, ttl time.Duration, now func() time.Time) error {
	res := db.Model(&SchedulerLock{}).
		Where("job_name = ? AND holder_id = ?", jobName, holderID).
		Update("expires_at", now().UTC().Add(ttl))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrLockLost
	}
	return nil
}

// release deletes the lock row owned by holderID. Idempotent.
func release(db *gorm.DB, jobName, holderID string) error {
	return db.Where("job_name = ? AND holder_id = ?", jobName, holderID).
		Delete(&SchedulerLock{}).Error
}
