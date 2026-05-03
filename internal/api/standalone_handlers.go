package api

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/mobazha/mobazha3.0/internal/version"
)

type healthResponse struct {
	Status               string `json:"status"`
	UnreadNotifications  int    `json:"unreadNotifications"`
}

func (g *Gateway) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	resp := healthResponse{Status: "ok"}

	if defaultNode := g.nodeManager.GetDefaultNode(); defaultNode != nil {
		if ns := defaultNode.Notification(); ns != nil {
			if unread, err := ns.GetNotificationsUnreadCount(); err == nil {
				resp.UnreadNotifications = unread
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type versionResponse struct {
	Version string `json:"version"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func (g *Gateway) handleAdminVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versionResponse{
		Version: version.String(),
		Go:      runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	})
}
