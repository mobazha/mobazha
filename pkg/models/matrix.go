package models

import "time"

// MatrixCredentials stores Matrix login credentials for the node.
// Password is derived from node private key using HKDF.
type MatrixCredentials struct {
	TenantID     string    `gorm:"column:tenant_id;uniqueIndex:idx_cred_tenant_peer;default:''" json:"-"`
	ID           uint      `gorm:"primaryKey" json:"id"`
	PeerID       string    `gorm:"uniqueIndex:idx_cred_tenant_peer;not null" json:"peerId"`
	MatrixUserID string    `gorm:"not null" json:"matrixUserId"`
	ServerName   string    `gorm:"size:255;not null" json:"serverName"`
	Registered   bool      `gorm:"default:false" json:"registered"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// MatrixRegisterRequest is the request body for registering Matrix account.
type MatrixRegisterRequest struct {
	PeerID      string `json:"peerId"`
	DisplayName string `json:"displayName,omitempty"`
}

// MatrixRegisterResponse is the response for registration.
type MatrixRegisterResponse struct {
	MatrixUserID  string `json:"matrixUserId"`
	ServerName    string `json:"serverName"`
	HomeserverURL string `json:"homeserverUrl"`
	Registered    bool   `json:"registered"`
}
