package contracts

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrVerificationUnavailable is returned when SAS/device verification is not wired (e.g. no crypto helper).
var ErrVerificationUnavailable = errors.New("verification not available")

// MatrixChatService provides full Matrix chat capabilities via the node backend.
// The frontend communicates only through REST API and WebSocket events; all Matrix
// protocol interaction (including E2EE) is handled by the node.
//
// This interface coexists with the legacy ChatService (P2P).
type MatrixChatService interface {
	// Start initializes the Matrix client, logs in, and begins syncing.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the Matrix client and sync loop.
	Stop() error
	// IsReady returns true when the client is logged in and syncing.
	IsReady() bool

	// --- Rooms ---

	// GetRooms returns all joined rooms with summary metadata.
	GetRooms(ctx context.Context) ([]MatrixRoom, error)
	// GetRoom returns detailed info for a single room.
	GetRoom(ctx context.Context, roomID string) (*MatrixRoom, error)
	// GetInvitedRooms returns rooms where the user has a pending invite.
	GetInvitedRooms(ctx context.Context) ([]MatrixRoom, error)
	// CreateDirectRoom creates or retrieves a 1:1 DM room with the given Matrix user.
	CreateDirectRoom(ctx context.Context, userID string) (string, error)
	// CreateGroupRoom creates a new group chat room.
	// metadata is optional key-value pairs stored as room state events (e.g. room type, order ID).
	CreateGroupRoom(ctx context.Context, name string, memberIDs []string, metadata map[string]string) (string, error)
	// JoinRoom joins an existing room by ID or alias.
	JoinRoom(ctx context.Context, roomID string) error
	// LeaveRoom leaves a room.
	LeaveRoom(ctx context.Context, roomID string) error
	// InviteToRoom invites a Matrix user to a room.
	InviteToRoom(ctx context.Context, roomID, userID string) error
	// KickUser removes a user from a room with an optional reason.
	KickUser(ctx context.Context, roomID, userID, reason string) error
	// SetRoomName changes the display name of a room.
	SetRoomName(ctx context.Context, roomID, name string) error
	// SetRoomTopic changes the topic of a room.
	SetRoomTopic(ctx context.Context, roomID, topic string) error
	// SetRoomAvatar uploads an image and sets it as the room avatar.
	SetRoomAvatar(ctx context.Context, roomID string, reader io.Reader, contentType string) error

	// --- Messages ---

	// SendMessage sends a text message to a room. Returns the event ID.
	SendMessage(ctx context.Context, roomID, content string) (string, error)
	// SendImage uploads and sends an image message. Returns the event ID.
	SendImage(ctx context.Context, roomID string, reader io.Reader, filename string, size int64) (string, error)
	// SendFile uploads and sends a file message. Returns the event ID.
	SendFile(ctx context.Context, roomID string, reader io.Reader, filename string, size int64) (string, error)
	// GetMessages returns paginated messages for a room.
	// `token` is an opaque pagination token. `dir` controls direction:
	//   "b" (default) = backward (older messages), "f" = forward (newer messages).
	GetMessages(ctx context.Context, roomID string, limit int, token string, dir string) ([]MatrixMessage, string, error)
	// EditMessage edits a previously sent message.
	EditMessage(ctx context.Context, roomID, eventID, newContent string) error
	// RedactMessage redacts (deletes) a message.
	RedactMessage(ctx context.Context, roomID, eventID string) error
	// SendReaction sends an emoji reaction to a message. Returns the reaction event ID.
	SendReaction(ctx context.Context, roomID, eventID, key string) (string, error)

	// --- Real-time ---

	// SendTyping sends a typing notification to a room.
	SendTyping(ctx context.Context, roomID string, typing bool) error
	// MarkAsRead marks the latest event in a room as read.
	MarkAsRead(ctx context.Context, roomID, eventID string) error

	// --- Events ---

	// Subscribe returns a channel that receives real-time chat events.
	// The caller must drain the channel to avoid blocking.
	Subscribe(ctx context.Context) (<-chan MatrixChatEvent, error)

	// --- Profile ---

	// SetDisplayName sets the user's Matrix display name.
	SetDisplayName(ctx context.Context, name string) error
	// SetAvatar sets the user's Matrix avatar.
	SetAvatar(ctx context.Context, reader io.Reader, contentType string) error

	// --- Media ---

	// DownloadMedia downloads a media file from the Matrix homeserver.
	// Returns the reader, content type, and content length.
	DownloadMedia(ctx context.Context, serverName, mediaID string) (io.ReadCloser, string, int64, error)

	// --- Block ---

	// BlockUser adds a user to the ignore list (m.ignored_user_list).
	BlockUser(ctx context.Context, userID string) error
	// UnblockUser removes a user from the ignore list.
	UnblockUser(ctx context.Context, userID string) error
	// GetBlockedUsers returns the list of blocked Matrix user IDs.
	GetBlockedUsers(ctx context.Context) ([]string, error)

	// --- Settings ---

	// GetChatSettings returns the current chat settings (invite policy, etc.).
	GetChatSettings(ctx context.Context) (*ChatSettings, error)
	// SetChatSettings updates chat settings and persists to Matrix account data.
	SetChatSettings(ctx context.Context, settings *ChatSettings) error

	// --- Verification ---

	// StartVerification sends a verification request to the given Matrix user.
	// Returns the transaction ID for tracking the verification flow.
	StartVerification(ctx context.Context, userID string) (string, error)
	// AcceptVerification accepts an incoming verification request.
	AcceptVerification(ctx context.Context, txnID string) error
	// StartSAS initiates the SAS (emoji) verification for an accepted transaction.
	StartSAS(ctx context.Context, txnID string) error
	// ConfirmSAS confirms the displayed SAS emoji/decimals match.
	ConfirmSAS(ctx context.Context, txnID string) error
	// CancelVerification cancels a pending or in-progress verification.
	CancelVerification(ctx context.Context, txnID string) error

	// --- Status ---

	// GetStatus returns the current connection status.
	// If the service is idle-paused, it will attempt to resume before returning status.
	GetStatus(ctx context.Context) MatrixChatStatus
}

// MatrixRoom represents a Matrix room with summary metadata.
type MatrixRoom struct {
	RoomID      string            `json:"roomId"`
	Name        string            `json:"name"`
	Topic       string            `json:"topic,omitempty"`
	AvatarURL   string            `json:"avatarUrl,omitempty"`
	RoomType    string            `json:"roomType"` // "direct", "group", "order", "store"
	IsDirect    bool              `json:"isDirect"`
	Members     []MatrixMember    `json:"members,omitempty"`
	LastMessage *MatrixMessage    `json:"lastMessage,omitempty"`
	UnreadCount int               `json:"unreadCount"`
	Encrypted   bool              `json:"encrypted"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// MatrixMember represents a room member.
type MatrixMember struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	PeerID      string `json:"peerID,omitempty"`
	Membership  string `json:"membership"` // "join", "invite", "leave", "ban"
}

// MatrixMessage represents a chat message.
type MatrixMessage struct {
	EventID   string            `json:"id"`
	RoomID    string            `json:"roomId"`
	Sender    string            `json:"sender"`
	Content   string            `json:"content"`
	MsgType   string            `json:"msgType"` // "m.text", "m.image", "m.file", etc.
	Timestamp time.Time         `json:"timestamp"`
	EditedAt  *time.Time        `json:"editedAt,omitempty"`
	ReplyTo   string            `json:"replyTo,omitempty"`
	Media     *MatrixMediaInfo  `json:"media,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// MatrixMediaInfo holds media attachment metadata.
type MatrixMediaInfo struct {
	URL         string `json:"url"`
	MimeType    string `json:"mimeType"`
	Size        int64  `json:"size"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Filename    string `json:"filename"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

// MatrixChatEvent is a real-time event pushed via WebSocket.
type MatrixChatEvent struct {
	Type string      `json:"type"` // "chat.message", "chat.typing", "chat.read_receipt", etc.
	Data interface{} `json:"data"`
}

// InvitePolicy controls how room invitations are handled.
type InvitePolicy string

const (
	InvitePolicyAutoAll      InvitePolicy = "auto_all"
	InvitePolicyAutoMobazha  InvitePolicy = "auto_mobazha"
	InvitePolicyAlwaysConfirm InvitePolicy = "always_confirm"
)

// ChatSettings holds user-configurable chat preferences.
type ChatSettings struct {
	InvitePolicy InvitePolicy `json:"invitePolicy"`
}

// MatrixChatStatus represents the connection status.
type MatrixChatStatus struct {
	Connected   bool   `json:"connected"`
	UserID      string `json:"userId,omitempty"`
	DeviceID    string `json:"deviceId,omitempty"`
	ServerName  string `json:"serverName,omitempty"`
	SyncRunning bool   `json:"syncRunning"`
}
