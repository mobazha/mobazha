package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/danielgtaylor/huma/v2"
)

func (g *Gateway) registerNodeHumaChatOperations(api huma.API) {
	// Rooms
	g.registerChatListRooms(api)
	g.registerChatListInvites(api)
	g.registerChatCreateRoom(api)
	g.registerChatJoinRoom(api)
	g.registerChatLeaveRoom(api)

	// Messages
	g.registerChatGetMessages(api)
	g.registerChatSendMessage(api)
	g.registerChatEditMessage(api)
	g.registerChatDeleteMessage(api)
	g.registerChatReactMessage(api)

	// Room actions
	g.registerChatTyping(api)
	g.registerChatMarkRead(api)
	g.registerChatGetMembers(api)
	g.registerChatInviteMember(api)
	g.registerChatKickMember(api)

	// Room settings
	g.registerChatGetRoomSettings(api)
	g.registerChatPutRoomSettings(api)

	// Media
	g.registerChatMediaDownload(api)
	g.registerChatSetRoomAvatar(api)
	g.registerChatMediaUpload(api)

	// Block
	g.registerChatBlockUser(api)
	g.registerChatUnblockUser(api)
	g.registerChatListBlockedUsers(api)

	// Presence
	g.registerChatGetPresence(api)
	g.registerChatSetPresence(api)

	// Global settings
	g.registerChatGetSettings(api)
	g.registerChatPutSettings(api)

	// Verification
	g.registerChatVerificationRequest(api)
	g.registerChatVerificationAccept(api)
	g.registerChatVerificationStartSAS(api)
	g.registerChatVerificationConfirm(api)
	g.registerChatVerificationCancel(api)

	// Status
	g.registerChatGetStatus(api)
}

// --- helper types ---

type chatRoomInput struct {
	RoomID string `path:"roomID" doc:"Matrix room ID."`
}

type chatRoomEventInput struct {
	RoomID  string `path:"roomID" doc:"Matrix room ID."`
	EventID string `path:"eventID" doc:"Matrix event ID."`
}

type chatUserInput struct {
	UserID string `path:"userID" doc:"Matrix user ID."`
}

type chatTxnInput struct {
	TxnID string `path:"txnId" doc:"Verification transaction ID."`
}

type chatMediaInput struct {
	ServerName string `path:"serverName" doc:"Matrix server name."`
	MediaID    string `path:"mediaID" doc:"Matrix media ID."`
}

type chatBodyInput struct {
	Body json.RawMessage
}

type chatRoomBodyInput struct {
	RoomID string `path:"roomID" doc:"Matrix room ID."`
	Body   json.RawMessage
}

type chatRoomEventBodyInput struct {
	RoomID  string `path:"roomID" doc:"Matrix room ID."`
	EventID string `path:"eventID" doc:"Matrix event ID."`
	Body    json.RawMessage
}

type chatUserBodyInput struct {
	UserID string `path:"userID" doc:"Matrix user ID."`
	Body   json.RawMessage
}

type chatTxnBodyInput struct {
	TxnID string `path:"txnId" doc:"Verification transaction ID."`
	Body  json.RawMessage
}

// --- Rooms ---

func (g *Gateway) registerChatListRooms(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-list-rooms",
		Method:      http.MethodGet,
		Path:        "/v1/chat/rooms",
		Summary:     "List chat rooms",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		rr := httptest.NewRecorder()
		g.handleGETMatrixChatRooms(rr, nodeBridgeRequest(ctx, http.MethodGet, "/v1/chat/rooms", nil))
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatListInvites(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-list-invites",
		Method:      http.MethodGet,
		Path:        "/v1/chat/invites",
		Summary:     "List pending chat invites",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		rr := httptest.NewRecorder()
		g.handleGETMatrixChatInvites(rr, nodeBridgeRequest(ctx, http.MethodGet, "/v1/chat/invites", nil))
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatCreateRoom(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-create-room",
		Method:      http.MethodPost,
		Path:        "/v1/chat/rooms",
		Summary:     "Create a new chat room",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatBodyInput) (*nodeDataOutput, error) {
		rr := httptest.NewRecorder()
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/chat/rooms", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		g.handlePOSTMatrixChatRoom(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatJoinRoom(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-join-room",
		Method:      http.MethodPost,
		Path:        "/v1/chat/rooms/{roomID}/join",
		Summary:     "Join a chat room",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/join"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"roomID": in.RoomID})
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatRoomJoin(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatLeaveRoom(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-leave-room",
		Method:      http.MethodPost,
		Path:        "/v1/chat/rooms/{roomID}/leave",
		Summary:     "Leave a chat room",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/leave"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"roomID": in.RoomID})
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatRoomLeave(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Messages ---

func (g *Gateway) registerChatGetMessages(api huma.API) {
	type chatMessagesInput struct {
		RoomID string `path:"roomID" doc:"Matrix room ID."`
		Limit  string `query:"limit" required:"false" doc:"Max messages to return."`
		Before string `query:"before" required:"false" doc:"Pagination cursor: events before this token."`
		After  string `query:"after" required:"false" doc:"Pagination cursor: events after this token."`
		Since  string `query:"since" required:"false" doc:"Sync-style incremental cursor."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "chat-get-messages",
		Method:      http.MethodGet,
		Path:        "/v1/chat/rooms/{roomID}/messages",
		Summary:     "Get messages in a chat room",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatMessagesInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/messages"
		v := url.Values{}
		if in.Limit != "" {
			v.Set("limit", in.Limit)
		}
		if in.Before != "" {
			v.Set("before", in.Before)
		}
		if in.After != "" {
			v.Set("after", in.After)
		}
		if in.Since != "" {
			v.Set("since", in.Since)
		}
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"roomID": in.RoomID})
		rr := httptest.NewRecorder()
		g.handleGETMatrixChatRoomMessages(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatSendMessage(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-send-message",
		Method:      http.MethodPost,
		Path:        "/v1/chat/rooms/{roomID}/messages",
		Summary:     "Send a message in a chat room",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/messages"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"roomID": in.RoomID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatRoomMessage(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatEditMessage(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-edit-message",
		Method:      http.MethodPut,
		Path:        "/v1/chat/rooms/{roomID}/messages/{eventID}",
		Summary:     "Edit a chat message",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomEventBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/messages/" + url.PathEscape(in.EventID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(in.Body), map[string]string{"roomID": in.RoomID, "eventID": in.EventID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTMatrixChatRoomMessage(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatDeleteMessage(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-delete-message",
		Method:      http.MethodDelete,
		Path:        "/v1/chat/rooms/{roomID}/messages/{eventID}",
		Summary:     "Delete a chat message",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomEventInput) (*nodeNoContentOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/messages/" + url.PathEscape(in.EventID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"roomID": in.RoomID, "eventID": in.EventID})
		rr := httptest.NewRecorder()
		g.handleDELETEMatrixChatRoomMessage(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (g *Gateway) registerChatReactMessage(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-react-message",
		Method:      http.MethodPost,
		Path:        "/v1/chat/rooms/{roomID}/messages/{eventID}/reactions",
		Summary:     "React to a chat message",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomEventBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/messages/" + url.PathEscape(in.EventID) + "/reactions"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"roomID": in.RoomID, "eventID": in.EventID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatRoomReaction(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Room actions ---

func (g *Gateway) registerChatTyping(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-typing",
		Method:      http.MethodPost,
		Path:        "/v1/chat/rooms/{roomID}/typing",
		Summary:     "Send typing indicator",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/typing"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"roomID": in.RoomID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatRoomTyping(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatMarkRead(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-mark-read",
		Method:      http.MethodPost,
		Path:        "/v1/chat/rooms/{roomID}/read",
		Summary:     "Mark room messages as read",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/read"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"roomID": in.RoomID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatRoomRead(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatGetMembers(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-get-members",
		Method:      http.MethodGet,
		Path:        "/v1/chat/rooms/{roomID}/members",
		Summary:     "List room members",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/members"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"roomID": in.RoomID})
		rr := httptest.NewRecorder()
		g.handleGETMatrixChatRoomMembers(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatInviteMember(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-invite-member",
		Method:      http.MethodPost,
		Path:        "/v1/chat/rooms/{roomID}/invite",
		Summary:     "Invite a user to a chat room",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/invite"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"roomID": in.RoomID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatRoomInvite(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatKickMember(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-kick-member",
		Method:      http.MethodPost,
		Path:        "/v1/chat/rooms/{roomID}/kick",
		Summary:     "Kick a user from a chat room",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/kick"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"roomID": in.RoomID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatRoomKick(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Room settings ---

func (g *Gateway) registerChatGetRoomSettings(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-get-room-settings",
		Method:      http.MethodGet,
		Path:        "/v1/chat/rooms/{roomID}/settings",
		Summary:     "Get chat room settings",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/settings"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"roomID": in.RoomID})
		rr := httptest.NewRecorder()
		g.handleGETMatrixChatRoomSettings(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatPutRoomSettings(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-put-room-settings",
		Method:      http.MethodPut,
		Path:        "/v1/chat/rooms/{roomID}/settings",
		Summary:     "Update chat room settings",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatRoomBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/rooms/" + url.PathEscape(in.RoomID) + "/settings"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(in.Body), map[string]string{"roomID": in.RoomID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTMatrixChatRoomSettings(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Media ---

func (g *Gateway) registerChatMediaDownload(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-media-download",
		Method:      http.MethodGet,
		Path:        "/v1/chat/media/{serverName}/{mediaID}",
		Summary:     "Download chat media",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatMediaInput) (*nodeLegacyBinaryBody, error) {
		rawURL := "/v1/chat/media/" + url.PathEscape(in.ServerName) + "/" + url.PathEscape(in.MediaID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"serverName": in.ServerName, "mediaID": in.MediaID})
		rr := httptest.NewRecorder()
		g.handleGETMatrixChatMediaDownload(rr, req)
		return nodeBridgeRecorderBinary(rr)
	})
}

// --- Block ---

func (g *Gateway) registerChatBlockUser(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-block-user",
		Method:      http.MethodPost,
		Path:        "/v1/chat/users/{userID}/block",
		Summary:     "Block a chat user",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatUserBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/users/" + url.PathEscape(in.UserID) + "/block"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"userID": in.UserID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatUserBlock(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatUnblockUser(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-unblock-user",
		Method:      http.MethodDelete,
		Path:        "/v1/chat/users/{userID}/block",
		Summary:     "Unblock a chat user",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatUserInput) (*nodeNoContentOutput, error) {
		rawURL := "/v1/chat/users/" + url.PathEscape(in.UserID) + "/block"
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"userID": in.UserID})
		rr := httptest.NewRecorder()
		g.handleDELETEMatrixChatUserBlock(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (g *Gateway) registerChatListBlockedUsers(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-list-blocked-users",
		Method:      http.MethodGet,
		Path:        "/v1/chat/blocked-users",
		Summary:     "List blocked chat users",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		rr := httptest.NewRecorder()
		g.handleGETMatrixChatBlockedUsers(rr, nodeBridgeRequest(ctx, http.MethodGet, "/v1/chat/blocked-users", nil))
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Presence ---

func (g *Gateway) registerChatGetPresence(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-get-presence",
		Method:      http.MethodGet,
		Path:        "/v1/chat/presence",
		Summary:     "Get chat presence status",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		rr := httptest.NewRecorder()
		g.handleGETMatrixChatPresence(rr, nodeBridgeRequest(ctx, http.MethodGet, "/v1/chat/presence", nil))
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatSetPresence(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-set-presence",
		Method:      http.MethodPost,
		Path:        "/v1/chat/presence",
		Summary:     "Set chat presence status",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/chat/presence", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatPresence(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Global settings ---

func (g *Gateway) registerChatGetSettings(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-get-settings",
		Method:      http.MethodGet,
		Path:        "/v1/chat/settings",
		Summary:     "Get global chat settings",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		rr := httptest.NewRecorder()
		g.handleGETMatrixChatSettings(rr, nodeBridgeRequest(ctx, http.MethodGet, "/v1/chat/settings", nil))
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatPutSettings(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-put-settings",
		Method:      http.MethodPut,
		Path:        "/v1/chat/settings",
		Summary:     "Update global chat settings",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/chat/settings", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTMatrixChatSettings(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Verification ---

func (g *Gateway) registerChatVerificationRequest(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-verification-request",
		Method:      http.MethodPost,
		Path:        "/v1/chat/verification/request",
		Summary:     "Request device verification",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/chat/verification/request", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatVerificationRequest(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatVerificationAccept(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-verification-accept",
		Method:      http.MethodPost,
		Path:        "/v1/chat/verification/{txnId}/accept",
		Summary:     "Accept verification request",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatTxnBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/verification/" + url.PathEscape(in.TxnID) + "/accept"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"txnId": in.TxnID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatVerificationAccept(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatVerificationStartSAS(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-verification-start-sas",
		Method:      http.MethodPost,
		Path:        "/v1/chat/verification/{txnId}/start-sas",
		Summary:     "Start SAS verification",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatTxnBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/verification/" + url.PathEscape(in.TxnID) + "/start-sas"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"txnId": in.TxnID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatVerificationStartSAS(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatVerificationConfirm(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-verification-confirm",
		Method:      http.MethodPost,
		Path:        "/v1/chat/verification/{txnId}/confirm",
		Summary:     "Confirm SAS verification",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatTxnBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/verification/" + url.PathEscape(in.TxnID) + "/confirm"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"txnId": in.TxnID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatVerificationConfirm(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatVerificationCancel(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-verification-cancel",
		Method:      http.MethodPost,
		Path:        "/v1/chat/verification/{txnId}/cancel",
		Summary:     "Cancel verification",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *chatTxnBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/chat/verification/" + url.PathEscape(in.TxnID) + "/cancel"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"txnId": in.TxnID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatVerificationCancel(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Status ---

func (g *Gateway) registerChatGetStatus(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat-get-status",
		Method:      http.MethodGet,
		Path:        "/v1/chat/status",
		Summary:     "Get Matrix chat connection status",
		Tags:        []string{"chat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		rr := httptest.NewRecorder()
		g.handleGETMatrixChatStatus(rr, nodeBridgeRequest(ctx, http.MethodGet, "/v1/chat/status", nil))
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatSetRoomAvatar(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:  "chat-set-room-avatar",
		Method:       http.MethodPost,
		Path:         "/v1/chat/rooms/{roomID}/avatar",
		Summary:      "Set chat room avatar (multipart)",
		Tags:         []string{"chat"},
		Security:     nodeAuthSecurity,
		MaxBodyBytes: 10 << 20,
	}, func(ctx context.Context, in *nodeMultipartWithRoomInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, "/v1/chat/rooms/"+in.RoomID+"/avatar",
			bytes.NewReader(in.RawBody), map[string]string{"roomID": in.RoomID})
		req.Header.Set("Content-Type", in.ContentType)
		req.ContentLength = int64(len(in.RawBody))
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatRoomAvatar(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerChatMediaUpload(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:  "chat-media-upload",
		Method:       http.MethodPost,
		Path:         "/v1/chat/media/upload",
		Summary:      "Upload chat media (multipart)",
		Tags:         []string{"chat"},
		Security:     nodeAuthSecurity,
		MaxBodyBytes: 50 << 20,
	}, func(ctx context.Context, in *nodeMultipartInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/chat/media/upload", bytes.NewReader(in.RawBody))
		req.Header.Set("Content-Type", in.ContentType)
		req.ContentLength = int64(len(in.RawBody))
		rr := httptest.NewRecorder()
		g.handlePOSTMatrixChatMediaUpload(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
