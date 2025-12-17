package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket 消息类型
type wsMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type connection struct {
	// The websocket connection
	ws *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// The hub
	h *hub
}

func (c *connection) reader() {
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if !websocket.IsUnexpectedCloseError(err) {
				log.Errorf("Websocket read error, nodeID: %s, error: %s", c.h.nodeID, err.Error())
			}
			break
		}

		// 尝试解析消息以检查是否是 ping
		var msg wsMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			if msg.Type == "ping" {
				// 响应应用级 pong（类似 converse.js/XMPP 的心跳）
				pong := []byte(`{"type":"pong"}`)
				select {
				case c.send <- pong:
					// pong 响应成功，不需要日志
				default:
					log.Warningf("Failed to send pong, channel full, nodeID: %s", c.h.nodeID)
				}
				continue
			}
		}

		// 其他消息回显到所有连接
		c.h.Broadcast <- message
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for message := range c.send {
		err := c.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Errorf("Websocket write error, nodeID: %s, error: %s", c.h.nodeID, err.Error())
			break
		}
	}
	c.ws.Close()
}

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type hub struct {
	nodeID string

	// Registered connections
	connections map[*connection]bool

	// Outbound messages to the connections
	Broadcast chan []byte

	// Register requests from the connections
	register chan *connection

	// Unregister requests from connections
	unregister chan *connection
}

func newHub(nodeID string) *hub {
	return &hub{
		nodeID:      nodeID,
		Broadcast:   make(chan []byte),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		connections: make(map[*connection]bool),
	}
}

func (h *hub) run() {
	// 协议级 ping 间隔（用于保持 nginx 等代理的连接）
	protocolPingTicker := time.NewTicker(45 * time.Second)
	defer func() {
		protocolPingTicker.Stop()
	}()

	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
			log.Debugf("Registered new websocket connection, nodeID: %s, total connections: %d", h.nodeID, len(h.connections))
		case c := <-h.unregister:
			if _, ok := h.connections[c]; ok {
				delete(h.connections, c)
				close(c.send)
			}
			log.Debugf("Unregistered websocket connection, nodeID: %s, remaining connections: %d", h.nodeID, len(h.connections))
		case m := <-h.Broadcast:
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					// 发送失败，可能是客户端断开，清理连接
					log.Warningf("Failed to send message to connection, removing, nodeID: %s", h.nodeID)
					delete(h.connections, c)
					close(c.send)
				}
			}
		// 协议级 WebSocket ping（用于保持代理连接，如 nginx）
		// 参考: https://nginx.org/en/docs/http/websocket.html
		// "By default, the connection will be closed if the proxied server does not transmit any data within 60 seconds."
		case <-protocolPingTicker.C:
			for c := range h.connections {
				if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Warningf("Protocol ping failed, nodeID: %s, error: %s", h.nodeID, err.Error())
					// 不在这里删除连接，让 reader 处理关闭
				}
			}
		}
	}
}

type websocketHandler struct {
	hub *hub
}

func newWebsocketHandler(hub *hub) *websocketHandler {
	handler := websocketHandler{
		hub: hub,
	}
	return &handler
}

func (wsh websocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("Error upgrading websocket, nodeID: %s, error: %s", wsh.hub.nodeID, err)
		return
	}
	c := &connection{send: make(chan []byte, 256), ws: ws, h: wsh.hub}
	c.h.register <- c
	defer func() { c.h.unregister <- c }()
	go c.writer()
	c.reader()
}
