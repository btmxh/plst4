package services

import (
	"errors"
	"html/template"
	"log/slog"
	"strings"
	"sync"

	"github.com/btmxh/plst4/internal/errs"
	"github.com/btmxh/plst4/internal/html"
	"github.com/btmxh/plst4/internal/media"
	"github.com/dchest/uniuri"
	"golang.org/x/net/websocket"
)

type WebSocketConnection struct {
	Client   RealtimeClient
	Username string
}

type RealtimeClient interface {
	Send(WebSocketMsg) error
}

type RealtimeClientFunc func(WebSocketMsg) error

func (f RealtimeClientFunc) Send(msg WebSocketMsg) error {
	return f(msg)
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

var GenericError = errors.New("Internal server error.")

type MediaChangedPayload struct {
	Type        media.MediaKind `json:"type"`
	Url         string          `json:"url"`
	AspectRatio string          `json:"aspectRatio"`
	NewVersion  int             `json:"newVersion"`
}

type WebSocketMsg struct {
	Type    WebSocketMsgType `json:"type"`
	Payload interface{}      `json:"payload"`
}

type WebSocketManager struct {
	playlists map[int]*PlaylistState
	sockets   map[string]RealtimeClient
	mutex     sync.RWMutex
}

type PlaylistState struct {
	userSockets   map[string]map[string]RealtimeClient
	nextRequested map[string]struct{}
}

func send(id string, client RealtimeClient, msg WebSocketMsg) {
	err := client.Send(msg)
	if err != nil {
		slog.Warn("Unable to send realtime message to client", "sid", id, "msg", msg)
	}
}

func (p *PlaylistState) Add(client RealtimeClient, username, id string) {
	if _, ok := p.userSockets[username]; !ok {
		p.userSockets[username] = make(map[string]RealtimeClient)
	}

	p.userSockets[username][id] = client
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
	sockets:   make(map[string]RealtimeClient),
}

func (manager *WebSocketManager) Add(conn *websocket.Conn, playlist int, username string) string {
	return manager.AddClient(RealtimeClientFunc(func(msg WebSocketMsg) error {
		return websocket.JSON.Send(conn, msg)
	}), playlist, username)
}

func (manager *WebSocketManager) AddClient(client RealtimeClient, playlist int, username string) string {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	id := uniuri.New()
	if _, ok := manager.playlists[playlist]; !ok {
		manager.playlists[playlist] = &PlaylistState{
			userSockets:   make(map[string]map[string]RealtimeClient),
			nextRequested: make(map[string]struct{}),
		}
	}

	manager.playlists[playlist].Add(client, username, id)
	manager.sockets[id] = client
	send(id, client, WebSocketMsg{Type: Handshake, Payload: id})
	return id
}

func (manager *WebSocketManager) Remove(playlist int, username string, id string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if manager.playlists[playlist].Remove(username, id) {
		delete(manager.playlists, playlist)
	}
	delete(manager.sockets, id)
}

func (manager *WebSocketManager) SendId(id string, msg WebSocketMsg) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	if socket, ok := manager.sockets[id]; ok {
		send(id, socket, msg)
	}
}

func (manager *WebSocketManager) BroadcastPlaylist(id int, msg WebSocketMsg) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	if p, ok := manager.playlists[id]; ok {
		p.Broadcast(msg)
	}
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

func GetManager() *WebSocketManager {
	return &manager
}

func NewWebSocketErrorHandler(title string, wsId string) errs.ErrorHandler {
	return errs.NewLogErrorHandler(title, func(err error) error {
		return WebSocketToast(wsId, html.ToastError, html.StringAsHTML(title), html.StringAsHTML(err.Error()))
	})
}

func WebSocketToast(socketId string, kind html.ToastKind, title template.HTML, description template.HTML) error {
	var str strings.Builder
	if err := html.RenderToast(&str, kind, title, description); err != nil {
		slog.Warn("error rendering toast notification for WebSocket", "err", err)
		return err
	}

	WebSocketSwap(socketId, template.HTML(str.String()))
	return nil
}
