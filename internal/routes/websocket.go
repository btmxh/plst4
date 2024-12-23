package routes

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/media"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/dchest/uniuri"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

type WebSocketConnection struct {
	Conn     *websocket.Conn
	Username string
}

type WebSocketMsgType string
type WebSocketEventType string

const (
	Handshake    WebSocketMsgType = "handshake"
	Swap         WebSocketMsgType = "swap"
	Event        WebSocketMsgType = "event"
	MediaChanged WebSocketMsgType = "media-change"

	ManagersChanged WebSocketEventType = "refresh-managers"
	PlaylistChanged WebSocketEventType = "refresh-playlist"
)

type MediaChangedPayload struct {
	Type        media.MediaKind `json:"type"`
	Url         string          `json:"url"`
	AspectRatio string          `json:"aspectRatio"`
}

type WebSocketMsg struct {
	Type    WebSocketMsgType `json:"type"`
	Payload interface{}      `json:"payload"`
}

type WebSocketManager struct {
	playlists map[int]*PlaylistState
	sockets   map[string]*websocket.Conn
}

type PlaylistState struct {
	userSockets   map[string]map[string]*websocket.Conn
	nextRequested map[string]struct{}
}

func send(id string, ws *websocket.Conn, msg WebSocketMsg) {
	err := websocket.JSON.Send(ws, msg)
	if err != nil {
		slog.Warn("Unable to send WebSocket message to client", "sid", id, "msg", msg)
	}
}

func (p *PlaylistState) Add(conn *websocket.Conn, username, id string) {
	if _, ok := p.userSockets[username]; !ok {
		p.userSockets[username] = make(map[string]*websocket.Conn)
	}

	p.userSockets[username][id] = conn
}

func (p *PlaylistState) Remove(username, id string) bool {
	delete(p.userSockets[username], id)
	if len(p.userSockets[username]) == 0 {
		delete(p.userSockets, username)
	}
	return len(p.userSockets) == 0
}

func (p *PlaylistState) NumManagerWatching() int {
	num := len(p.userSockets)
	if _, ok := p.userSockets[""]; ok {
		num -= 1
	}
	return num
}

func (p *PlaylistState) Broadcast(msg WebSocketMsg) {
	for _, sockets := range p.userSockets {
		for socketId, socket := range sockets {
			send(socketId, socket, msg)
		}
	}
}

var manager WebSocketManager = WebSocketManager{
	playlists: make(map[int]*PlaylistState),
	sockets:   make(map[string]*websocket.Conn),
}

func (manager *WebSocketManager) NumManagerWatching(playlist int) int {
	if p, ok := manager.playlists[playlist]; ok {
		return p.NumManagerWatching()
	}

	return 0
}

func (manager *WebSocketManager) Add(conn *websocket.Conn, playlist int, username string) string {
	id := uniuri.New()
	if _, ok := manager.playlists[playlist]; !ok {
		manager.playlists[playlist] = &PlaylistState{
			userSockets:   make(map[string]map[string]*websocket.Conn),
			nextRequested: make(map[string]struct{}),
		}
	}

	manager.playlists[playlist].Add(conn, username, id)
	manager.sockets[id] = conn
	send(id, conn, WebSocketMsg{Type: Handshake, Payload: id})
	return id
}

func (manager *WebSocketManager) Remove(playlist int, username string, id string) {
	if manager.playlists[playlist].Remove(username, id) {
		delete(manager.playlists, playlist)
	}
}

func (manager *WebSocketManager) SendId(id string, msg WebSocketMsg) {
	if socket, ok := manager.sockets[id]; ok {
		send(id, socket, msg)
	}
}

func (manager *WebSocketManager) BroadcastPlaylist(id int, msg WebSocketMsg) {
	if p, ok := manager.playlists[id]; ok {
		p.Broadcast(msg)
	}
}

func WebSocketRouter(g *gin.RouterGroup) {
	g.GET("/:id", func(c *gin.Context) {
		playlist, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		username, _ := middlewares.GetAuthUsername(c)
		websocket.Handler(func(conn *websocket.Conn) {
			defer conn.Close()

			socketId := manager.Add(conn, playlist, username)
			defer manager.Remove(playlist, username, socketId)

			var payload MediaChangedPayload

			err = db.DB.QueryRow("SELECT m.media_type, m.url, m.aspect_ratio FROM playlists p JOIN playlist_items i ON p.current = i.id JOIN medias m ON m.id = i.media WHERE p.id = $1", playlist).Scan(&payload.Type, &payload.Url, &payload.AspectRatio)
			if err != nil && err != sql.ErrNoRows {
				slog.Warn("Error querying current playlist item", "err", err)
				return
			}

			if err == nil {
				WebSocketMediaChange(-1, socketId, payload)
			}

			for {
				var msg WebSocketMsg
				err = websocket.JSON.Receive(conn, msg)
				if err != nil {
					slog.Info("WebSocket connection closed or error", "err", err)
					break
				}

				slog.Debug("Received from WebSocket", "id", playlist, "msg", msg)
			}
		}).ServeHTTP(c.Writer, c.Request)
	})
}

func WebSocketSwap(socketId string, html template.HTML) {
	manager.SendId(socketId, WebSocketMsg{
		Type:    Swap,
		Payload: string(html),
	})
}

func WebSocketEvent(socketId, event string) {
	manager.SendId(socketId, WebSocketMsg{
		Type:    Event,
		Payload: string(event),
	})
}

func WebSocketMediaChange(playlist int, socketId string, payload MediaChangedPayload) {
	msg := WebSocketMsg{Type: MediaChanged, Payload: payload}
	if socketId == "" {
		manager.BroadcastPlaylist(playlist, msg)
	} else {
		manager.SendId(socketId, msg)
	}
}

func WebSocketPlaylistEvent(playlist int, event WebSocketEventType) {
	manager.BroadcastPlaylist(playlist, WebSocketMsg{Type: Event, Payload: event})
}

func NextRequest(playlist int, username string) (template.HTML, error) {
	if p, ok := manager.playlists[playlist]; ok {
		p.nextRequested[username] = struct{}{}

		for username := range p.userSockets {
			if username == "" {
				continue
			}

			if _, ok := p.nextRequested[username]; !ok {
				return "", nil
			}
		}

		for k := range p.nextRequested {
			delete(p.nextRequested, k)
		}

		msg, err := PlaylistUpdateCurrent(playlist, ">", "ASC")
		return msg, err
	}

	return "", nil
}
