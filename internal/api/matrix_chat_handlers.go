package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

func (g *Gateway) handleGETMatrixChatRooms(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	rooms, err := svc.GetRooms(r.Context())
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.Success(w, rooms)
}

func (g *Gateway) handlePOSTMatrixChatRoom(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	var req struct {
		UserID    string   `json:"userID"`
		Name      string   `json:"name"`
		MemberIDs []string `json:"memberIDs"`
		IsDM      bool     `json:"isDM"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	var roomID string
	var err error
	if req.IsDM && req.UserID != "" {
		roomID, err = svc.CreateDirectRoom(r.Context(), req.UserID)
	} else {
		roomID, err = svc.CreateGroupRoom(r.Context(), req.Name, req.MemberIDs)
	}
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.Created(w, map[string]string{"roomId": roomID})
}

func (g *Gateway) handlePOSTMatrixChatRoomJoin(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	if err := svc.JoinRoom(r.Context(), roomID); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatRoomLeave(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	if err := svc.LeaveRoom(r.Context(), roomID); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleGETMatrixChatRoomMessages(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	limitStr := r.URL.Query().Get("limit")
	before := r.URL.Query().Get("before")
	since := r.URL.Query().Get("since")

	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}

	token := before
	if token == "" {
		token = since
	}

	messages, nextToken, err := svc.GetMessages(r.Context(), roomID, limit, token)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
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
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Body == "" {
		ErrorResponse(w, http.StatusBadRequest, "body is required")
		return
	}

	eventID, err := svc.SendMessage(r.Context(), roomID, req.Body)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.Created(w, map[string]string{"eventId": eventID})
}

func (g *Gateway) handlePUTMatrixChatRoomMessage(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	vars := mux.Vars(r)
	roomID := vars["roomID"]
	eventID := vars["eventID"]
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Body == "" {
		ErrorResponse(w, http.StatusBadRequest, "body is required")
		return
	}

	if err := svc.EditMessage(r.Context(), roomID, eventID, req.Body); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleDELETEMatrixChatRoomMessage(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	vars := mux.Vars(r)
	roomID := vars["roomID"]
	eventID := vars["eventID"]

	if err := svc.RedactMessage(r.Context(), roomID, eventID); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatRoomReaction(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	_ = mux.Vars(r)["roomID"]
	_ = mux.Vars(r)["eventID"]

	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = svc
	_ = req
	ErrorResponse(w, http.StatusNotImplemented, "reactions not yet implemented")
}

func (g *Gateway) handlePOSTMatrixChatRoomTyping(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		Typing bool `json:"typing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := svc.SendTyping(r.Context(), roomID, req.Typing); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatRoomRead(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		EventID string `json:"eventId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.EventID == "" {
		ErrorResponse(w, http.StatusBadRequest, "eventId is required")
		return
	}

	if err := svc.MarkAsRead(r.Context(), roomID, req.EventID); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handleGETMatrixChatRoomMembers(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	room, err := svc.GetRoom(r.Context(), roomID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.Success(w, room.Members)
}

func (g *Gateway) handlePOSTMatrixChatRoomInvite(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		UserID string `json:"userID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.UserID == "" {
		ErrorResponse(w, http.StatusBadRequest, "userID is required")
		return
	}

	if err := svc.InviteToRoom(r.Context(), roomID, req.UserID); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatRoomKick(w http.ResponseWriter, r *http.Request) {
	_ = getMatrixChatService(r)
	ErrorResponse(w, http.StatusNotImplemented, "kick not yet implemented")
}

func (g *Gateway) handleGETMatrixChatRoomSettings(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	room, err := svc.GetRoom(r.Context(), roomID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
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
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	roomID := mux.Vars(r)["roomID"]
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != "" {
		if err := svc.SetRoomName(r.Context(), roomID, req.Name); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	responsePkg.NoContent(w)
}

func (g *Gateway) handlePOSTMatrixChatMediaUpload(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "file too large or invalid form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	roomID := r.FormValue("roomID")
	if roomID == "" {
		ErrorResponse(w, http.StatusBadRequest, "roomID is required")
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
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	responsePkg.Created(w, map[string]string{"eventId": eventID})
}

func (g *Gateway) handleGETMatrixChatMediaDownload(w http.ResponseWriter, r *http.Request) {
	svc := getMatrixChatService(r)
	if svc == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "matrix chat service not available")
		return
	}
	vars := mux.Vars(r)
	serverName := vars["serverName"]
	mediaID := vars["mediaID"]

	reader, contentType, size, err := svc.DownloadMedia(r.Context(), serverName, mediaID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

func (g *Gateway) handlePOSTMatrixChatUserBlock(w http.ResponseWriter, r *http.Request) {
	ErrorResponse(w, http.StatusNotImplemented, "block not yet implemented")
}

func (g *Gateway) handleDELETEMatrixChatUserBlock(w http.ResponseWriter, r *http.Request) {
	ErrorResponse(w, http.StatusNotImplemented, "unblock not yet implemented")
}

func (g *Gateway) handleGETMatrixChatPresence(w http.ResponseWriter, r *http.Request) {
	ErrorResponse(w, http.StatusNotImplemented, "presence not yet implemented")
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
