package models

import "time"

// =====================================================
// Matrix Credentials (for direct Matrix login)
// =====================================================

// MatrixCredentials stores Matrix login credentials for the node
// Password is derived from node private key using HKDF
// This allows direct Matrix login without relying on hosting server
type MatrixCredentials struct {
	TenantID     string    `gorm:"column:tenant_id;uniqueIndex:idx_cred_tenant_peer;default:''" json:"-"`
	ID           uint      `gorm:"primaryKey" json:"id"`
	PeerID       string    `gorm:"uniqueIndex:idx_cred_tenant_peer;not null" json:"peerId"`  // Mobazha peer ID
	MatrixUserID string    `gorm:"not null" json:"matrixUserId"`        // Matrix user ID (e.g., @peer_xxx:matrix.mobazha.org)
	ServerName   string    `gorm:"size:255;not null" json:"serverName"` // Matrix server name (e.g., matrix.mobazha.org)
	Registered   bool      `gorm:"default:false" json:"registered"`     // Whether user has been registered on Matrix server
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// MatrixCredentialsResponse is the response for getting credentials
// Password is derived on-the-fly from node private key, not stored
type MatrixCredentialsResponse struct {
	MatrixUserID  string `json:"matrixUserId"`
	Password      string `json:"password"`
	ServerName    string `json:"serverName"`
	HomeserverURL string `json:"homeserverUrl"`
	Registered    bool   `json:"registered"`
}

// MatrixRegisterRequest is the request body for registering Matrix account
type MatrixRegisterRequest struct {
	PeerID      string `json:"peerId"`
	DisplayName string `json:"displayName,omitempty"`
}

// MatrixRegisterResponse is the response for registration
type MatrixRegisterResponse struct {
	MatrixUserID  string `json:"matrixUserId"`
	ServerName    string `json:"serverName"`
	HomeserverURL string `json:"homeserverUrl"`
	Registered    bool   `json:"registered"`
}

// =====================================================
// Matrix E2EE Key Backup (room keys)
// =====================================================

// MatrixKeyBackup stores encrypted Matrix E2EE key backups
// Keys are encrypted using a key derived from the node's private key
// All encryption/decryption happens server-side
type MatrixKeyBackup struct {
	TenantID      string    `gorm:"column:tenant_id;uniqueIndex:idx_backup_tenant_device;default:''" json:"-"`
	ID            uint      `gorm:"primaryKey" json:"id"`
	DeviceID      string    `gorm:"uniqueIndex:idx_backup_tenant_device;not null" json:"deviceId"` // Matrix device ID
	EncryptedKeys []byte    `gorm:"type:blob;not null" json:"-"`          // Encrypted room keys (not exposed in JSON)
	KeyCount      int       `json:"keyCount"`                             // Number of keys in backup
	Algorithm     string    `gorm:"size:50;not null;default:'aes-256-gcm'" json:"algorithm"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// MatrixKeyBackupRequest is the request body for saving a key backup
type MatrixKeyBackupRequest struct {
	DeviceID string `json:"deviceId"`
	Keys     string `json:"keys"` // JSON string of room keys (will be encrypted server-side)
}

// MatrixKeyBackupResponse is the response for getting a key backup
type MatrixKeyBackupResponse struct {
	DeviceID  string    `json:"deviceId"`
	Keys      string    `json:"keys"` // Decrypted room keys JSON
	KeyCount  int       `json:"keyCount"`
	Algorithm string    `json:"algorithm"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MatrixKeyBackupInfo is the response for getting backup info without decrypting
type MatrixKeyBackupInfo struct {
	DeviceID  string    `json:"deviceId"`
	KeyCount  int       `json:"keyCount"`
	Algorithm string    `json:"algorithm"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// =====================================================
// Matrix Secrets Bundle (cross-signing keys)
// =====================================================

// MatrixSecretsBundle stores encrypted Matrix cross-signing secrets bundle
// This includes Master Key, Self-Signing Key, User-Signing Key private keys
// Keys are encrypted using a key derived from the node's private key
// All encryption/decryption happens server-side
type MatrixSecretsBundle struct {
	TenantID         string    `gorm:"column:tenant_id;uniqueIndex:idx_secrets_tenant_device;default:''" json:"-"`
	ID               uint      `gorm:"primaryKey" json:"id"`
	DeviceID         string    `gorm:"uniqueIndex:idx_secrets_tenant_device;not null" json:"deviceId"` // Matrix device ID that created the backup
	EncryptedSecrets []byte    `gorm:"type:blob;not null" json:"-"`          // Encrypted secrets bundle (not exposed in JSON)
	Algorithm        string    `gorm:"size:50;not null;default:'aes-256-gcm'" json:"algorithm"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// MatrixSecretsBundleRequest is the request body for saving a secrets bundle
type MatrixSecretsBundleRequest struct {
	DeviceID string `json:"deviceId"`
	Secrets  string `json:"secrets"` // JSON string of secrets bundle (will be encrypted server-side)
}

// MatrixSecretsBundleResponse is the response for getting a secrets bundle
type MatrixSecretsBundleResponse struct {
	DeviceID  string    `json:"deviceId"`
	Secrets   string    `json:"secrets"` // Decrypted secrets bundle JSON
	Algorithm string    `json:"algorithm"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MatrixSecretsBundleInfo is the response for getting info without decrypting
type MatrixSecretsBundleInfo struct {
	Exists    bool      `json:"exists"`
	DeviceID  string    `json:"deviceId,omitempty"`
	Algorithm string    `json:"algorithm,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}
