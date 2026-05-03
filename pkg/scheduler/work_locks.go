package scheduler

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// WorkLock is the GORM model backing scheduler_work_locks. Each row
// represents a per-(job, tenant) lease used by NodeFn fan-out to prevent
// double-running the same tenant's work across multiple hosting instances.
//
// Composite primary key: (job_name, tenant_id).
// The schema is dialect-agnostic (SQLite + PostgreSQL) — only portable
// GORM operations are used.
type WorkLock struct {
	JobName    string    `gorm:"column:job_name;primaryKey;size:128"`
	TenantID   string    `gorm:"column:tenant_id;primaryKey;size:128"`
	HolderID   string    `gorm:"column:holder_id;size:128;not null"`
	AcquiredAt time.Time `gorm:"column:acquired_at;not null"`
	ExpiresAt  time.Time `gorm:"column:expires_at;index;not null"`
}

// TableName is the explicit table name for WorkLock.
func (WorkLock) TableName() string { return "scheduler_work_locks" }

// ErrWorkLockLost is returned by RenewWork when the holder no longer owns
// the work lock (e.g. another instance acquired it after expiry).
var ErrWorkLockLost = errors.New("scheduler: work lock lost")

// MigrateWorkLocks creates or upgrades the scheduler_work_locks table.
// Call once at process start alongside MigrateLocks, before Scheduler.Start().
func MigrateWorkLocks(db *gorm.DB) error {
	return db.AutoMigrate(&WorkLock{})
}

// TryClaimWork attempts to acquire a per-(job, tenant) work lock.
//
// Claim succeeds when:
//  1. No row exists for (jobName, tenantID) — insert; or
//  2. Existing row has expired (UPDATE WHERE expires_at < now); or
//  3. Existing row belongs to the same holderID (re-acquire / renew).
//
// Returns false when another live holder owns the lock.
func TryClaimWork(db *gorm.DB, jobName, tenantID, holderID string, ttl time.Duration, now func() time.Time) (bool, error) {
	t := now().UTC()
	expires := t.Add(ttl)

	res := db.Model(&WorkLock{}).
		Where("job_name = ? AND tenant_id = ? AND (expires_at < ? OR holder_id = ?)",
			jobName, tenantID, t, holderID).
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

	createErr := db.Create(&WorkLock{
		JobName:    jobName,
		TenantID:   tenantID,
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

// RenewWork extends the lease for (jobName, tenantID). Returns
// ErrWorkLockLost if holderID no longer owns the row.
func RenewWork(db *gorm.DB, jobName, tenantID, holderID string, ttl time.Duration, now func() time.Time) error {
	res := db.Model(&WorkLock{}).
		Where("job_name = ? AND tenant_id = ? AND holder_id = ?", jobName, tenantID, holderID).
		Update("expires_at", now().UTC().Add(ttl))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrWorkLockLost
	}
	return nil
}

// ReleaseWork deletes the work lock row owned by holderID. Idempotent.
func ReleaseWork(db *gorm.DB, jobName, tenantID, holderID string) error {
	return db.Where("job_name = ? AND tenant_id = ? AND holder_id = ?", jobName, tenantID, holderID).
		Delete(&WorkLock{}).Error
}

// LeaseRenewer periodically renews a work lock in the background.
// Call Stop() to terminate the renewal goroutine.
type LeaseRenewer struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// StartLeaseRenewer spawns a goroutine that renews the work lock every
// ttl/3. If renewal fails (lock lost or context cancelled), it cancels
// workCtx to signal the worker to stop.
func StartLeaseRenewer(
	ctx context.Context,
	workCancel context.CancelFunc,
	db *gorm.DB,
	jobName, tenantID, holderID string,
	ttl time.Duration,
	now func() time.Time,
) *LeaseRenewer {
	renewCtx, renewCancel := context.WithCancel(ctx)
	lr := &LeaseRenewer{
		cancel: renewCancel,
		done:   make(chan struct{}),
	}

	renewInterval := ttl / 3
	if renewInterval < time.Second {
		renewInterval = time.Second
	}

	go func() {
		defer close(lr.done)
		ticker := time.NewTicker(renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if err := RenewWork(db, jobName, tenantID, holderID, ttl, now); err != nil {
					workCancel()
					return
				}
			}
		}
	}()

	return lr
}

// Stop terminates the renewal goroutine and waits for it to finish.
func (lr *LeaseRenewer) Stop() {
	lr.cancel()
	<-lr.done
}
