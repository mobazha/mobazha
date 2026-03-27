package api

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

func (g *Gateway) handleGETMatrixChatRooms(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	rooms, err := svc.GetRooms(r.Context())
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Success(w, rooms)
}

func (g *Gateway) handleGETMatrixChatInvites(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}

	rooms, err := svc.GetInvitedRooms(r.Context())
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Success(w, rooms)
}

func (g *Gateway) handlePOSTMatrixChatRoom(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	var req struct {
		UserID    string            `json:"userID"`
		Name      string            `json:"name"`
		MemberIDs []string          `json:"memberIDs"`
		IsDM      bool              `json:"isDM"`
		Metadata  map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}

	var roomID string
	var err error
	if req.IsDM && req.UserID != "" {
		roomID, err = svc.CreateDirectRoom(r.Context(), req.UserID)
	} else {
		roomID, err = svc.CreateGroupRoom(r.Context(), req.Name, req.MemberIDs, req.Metadata)
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Created(w, map[string]string{"roomId": roomID})
}

func (g *Gateway) handlePOSTMatrixChatRoomJoin(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	if err := svc.JoinRoom(r.Context(), roomID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatRoomLeave(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	if err := svc.LeaveRoom(r.Context(), roomID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleGETMatrixChatRoomMessages(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	limitStr := r.URL.Query().Get("limit")
	before := r.URL.Query().Get("before")
	after := r.URL.Query().Get("after")
	since := r.URL.Query().Get("since")

	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}

	if before != "" && after != "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "cannot specify both before and after")
		return
	}

	var token, dir string
	switch {
	case before != "":
		token, dir = before, "b"
	case after != "":
		token, dir = after, "f"
	case since != "":
		token, dir = since, "b"
		w.Header().Set("X-Deprecated", "since parameter is deprecated; use before or after")
	default:
		dir = "b"
	}

	messages, nextToken, err := svc.GetMessages(r.Context(), roomID, limit, token, dir)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Success(w, map[string]interface{}{
		"messages": messages,
		"end":      nextToken,
	})
}

func (g *Gateway) handlePOSTMatrixChatRoomMessage(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if req.Body == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "body is required")
		return
	}

	eventID, err := svc.SendMessage(r.Context(), roomID, req.Body)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Created(w, map[string]string{"eventId": eventID})
}

func (g *Gateway) handlePUTMatrixChatRoomMessage(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	vars := mux.Vars(r)
	roomID := vars["roomID"]
	eventID := vars["eventID"]
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if req.Body == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "body is required")
		return
	}

	if err := svc.EditMessage(r.Context(), roomID, eventID, req.Body); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleDELETEMatrixChatRoomMessage(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	vars := mux.Vars(r)
	roomID := vars["roomID"]
	eventID := vars["eventID"]

	if err := svc.RedactMessage(r.Context(), roomID, eventID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatRoomReaction(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	vars := mux.Vars(r)
	roomID := vars["roomID"]
	eventID := vars["eventID"]

	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if req.Key == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "key is required")
		return
	}

	reactionEventID, err := svc.SendReaction(r.Context(), roomID, eventID, req.Key)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Created(w, map[string]string{"eventId": reactionEventID})
}

func (g *Gateway) handlePOSTMatrixChatRoomTyping(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		Typing bool `json:"typing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}

	if err := svc.SendTyping(r.Context(), roomID, req.Typing); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatRoomRead(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		EventID string `json:"eventId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if req.EventID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "eventId is required")
		return
	}

	if err := svc.MarkAsRead(r.Context(), roomID, req.EventID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleGETMatrixChatRoomMembers(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	room, err := svc.GetRoom(r.Context(), roomID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Success(w, room.Members)
}

func (g *Gateway) handlePOSTMatrixChatRoomInvite(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		UserID string `json:"userID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if req.UserID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "userID is required")
		return
	}

	if err := svc.InviteToRoom(r.Context(), roomID, req.UserID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatRoomKick(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		UserID string `json:"userID"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if req.UserID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "userID is required")
		return
	}
	if err := svc.KickUser(r.Context(), roomID, req.UserID, req.Reason); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleGETMatrixChatRoomSettings(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	room, err := svc.GetRoom(r.Context(), roomID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Success(w, map[string]interface{}{
		"roomId":    room.RoomID,
		"name":      room.Name,
		"topic":     room.Topic,
		"avatarUrl": room.AvatarURL,
		"encrypted": room.Encrypted,
		"roomType":  room.RoomType,
	})
}

func (g *Gateway) handlePUTMatrixChatRoomSettings(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		Name  string  `json:"name"`
		Topic *string `json:"topic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	if req.Name != "" {
		if err := svc.SetRoomName(r.Context(), roomID, req.Name); err != nil {
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
			return
		}
	}
	if req.Topic != nil {
		if err := svc.SetRoomTopic(r.Context(), roomID, *req.Topic); err != nil {
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
			return
		}
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatRoomAvatar(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "file too large or invalid form")
		return
	}
	file, header, err := r.FormFile("avatar")
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "avatar field is required")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := svc.SetRoomAvatar(r.Context(), roomID, file, contentType); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatMediaUpload(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "file too large or invalid form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "file field is required")
		return
	}
	defer file.Close()

	roomID := r.FormValue("roomId")
	if roomID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "roomId is required")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	isImage := contentType == "image/jpeg" || contentType == "image/png" ||
		contentType == "image/gif" || contentType == "image/webp"

	var eventID string
	if isImage {
		eventID, err = svc.SendImage(r.Context(), roomID, file, header.Filename, header.Size)
	} else {
		eventID, err = svc.SendFile(r.Context(), roomID, file, header.Filename, header.Size)
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Created(w, map[string]string{"eventId": eventID})
}

func (g *Gateway) handleGETMatrixChatMediaDownload(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	vars := mux.Vars(r)
	serverName := vars["serverName"]
	mediaID := vars["mediaID"]

	if !isValidMatrixServerName(serverName) {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "invalid server name")
		return
	}
	if strings.ContainsAny(mediaID, "/\\") {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "invalid media ID")
		return
	}

	reader, contentType, size, err := svc.DownloadMedia(r.Context(), serverName, mediaID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, reader); err != nil {
		log.Warningf("media proxy: incomplete copy for %s/%s: %v", serverName, mediaID, err)
	}
}

// isValidMatrixServerName validates a Matrix server name for SSRF prevention.
// Rejects IP addresses, port suffixes, path separators, and localhost.
func isValidMatrixServerName(s string) bool {
	if s == "" || len(s) > 255 {
		return false
	}
	if strings.ContainsAny(s, "/\\@") {
		return false
	}
	host := s
	if idx := strings.LastIndex(s, ":"); idx != -1 {
		host = s[:idx]
	}
	if net.ParseIP(host) != nil {
		return false
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return false
	}
	if !strings.Contains(host, ".") {
		return false
	}
	return true
}

func (g *Gateway) handlePOSTMatrixChatUserBlock(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	userID := mux.Vars(r)["userID"]
	if userID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "userID is required")
		return
	}
	if err := svc.BlockUser(r.Context(), userID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleDELETEMatrixChatUserBlock(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	userID := mux.Vars(r)["userID"]
	if userID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "userID is required")
		return
	}
	if err := svc.UnblockUser(r.Context(), userID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleGETMatrixChatBlockedUsers(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	users, err := svc.GetBlockedUsers(r.Context())
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Success(w, users)
}

func (g *Gateway) handleGETMatrixChatPresence(w http.ResponseWriter, r *http.Request) {
	responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "presence not yet implemented")
}

func (g *Gateway) handlePOSTMatrixChatPresence(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	var req struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "invalid request body")
		return
	}
	if req.DisplayName == "" {
		responsePkg.NoContent(w)
		return
	}
	if err := svc.SetDisplayName(r.Context(), req.DisplayName); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleGETMatrixChatSettings(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	settings, err := svc.GetChatSettings(r.Context())
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Success(w, settings)
}

func (g *Gateway) handlePUTMatrixChatSettings(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}

	var req struct {
		InvitePolicy string `json:"invitePolicy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "invalid request body")
		return
	}

	settings := &contracts.ChatSettings{
		InvitePolicy: contracts.InvitePolicy(req.InvitePolicy),
	}
	if err := svc.SetChatSettings(r.Context(), settings); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, err.Error())
		return
	}
	responsePkg.Success(w, settings)
}

func (g *Gateway) handleGETMatrixChatStatus(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Success(w, map[string]interface{}{
			"connected":   false,
			"syncRunning": false,
		})
		return
	}
	status := svc.GetStatus()
	responsePkg.Success(w, status)
}

// ===================== Verification Handlers =====================

func (g *Gateway) handlePOSTMatrixChatVerificationRequest(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}

	var req struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "userId is required")
		return
	}

	txnID, err := svc.StartVerification(r.Context(), req.UserID)
	if err != nil {
		if errors.Is(err, contracts.ErrVerificationUnavailable) {
			responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, err.Error())
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.Created(w, map[string]string{"transactionId": txnID})
}

func (g *Gateway) handlePOSTMatrixChatVerificationAccept(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	txnID := mux.Vars(r)["txnId"]
	if txnID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "txnId is required")
		return
	}
	if err := svc.AcceptVerification(r.Context(), txnID); err != nil {
		if errors.Is(err, contracts.ErrVerificationUnavailable) {
			responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, err.Error())
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatVerificationStartSAS(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	txnID := mux.Vars(r)["txnId"]
	if txnID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "txnId is required")
		return
	}
	if err := svc.StartSAS(r.Context(), txnID); err != nil {
		if errors.Is(err, contracts.ErrVerificationUnavailable) {
			responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, err.Error())
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatVerificationConfirm(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	txnID := mux.Vars(r)["txnId"]
	if txnID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "txnId is required")
		return
	}
	if err := svc.ConfirmSAS(r.Context(), txnID); err != nil {
		if errors.Is(err, contracts.ErrVerificationUnavailable) {
			responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, err.Error())
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatVerificationCancel(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		responsePkg.Error(w, http.StatusServiceUnavailable, responsePkg.CodeServiceUnavail, "matrix chat service not available")
		return
	}
	txnID := mux.Vars(r)["txnId"]
	if txnID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "txnId is required")
		return
	}
	if err := svc.CancelVerification(r.Context(), txnID); err != nil {
		if errors.Is(err, contracts.ErrVerificationUnavailable) {
			responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, err.Error())
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}
