package routes

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dchest/uniuri"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

type WebSocketConnection struct {
	Conn       *websocket.Conn
	PlaylistId int
}

type WebSocketMsgType string

const (
	Handshake WebSocketMsgType = "handshake"
	Swap      WebSocketMsgType = "swap"
	Event     WebSocketMsgType = "event"
)

type WebSocketMsg struct {
	Type    WebSocketMsgType `json:"type"`
	Payload string           `json:"payload"`
}

var socketConnections map[string]WebSocketConnection = make(map[string]WebSocketConnection)

func WebSocketRouter(g *gin.RouterGroup) {
	g.GET("/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		websocket.Handler(func(conn *websocket.Conn) {
			defer conn.Close()

			websocketIdentifier := uniuri.New()
			err := websocket.JSON.Send(conn, WebSocketMsg{
				Type:    Handshake,
				Payload: websocketIdentifier,
			})
			if err != nil {
				return
			}
			socketConnections[websocketIdentifier] = WebSocketConnection{
				Conn:       conn,
				PlaylistId: id,
			}
			defer delete(socketConnections, websocketIdentifier)

			for {
				var msg WebSocketMsg
				err = websocket.JSON.Receive(conn, msg)
				if err != nil {
					slog.Info("WebSocket connection closed or error", "err", err)
					break
				}

				slog.Debug("Received from WebSocket", "id", websocketIdentifier, "msg", msg)
			}
		}).ServeHTTP(c.Writer, c.Request)
	})
}

func WebSocketSwap(socketId string, html template.HTML) error {
	if conn, ok := socketConnections[socketId]; ok {
		return websocket.JSON.Send(conn.Conn, WebSocketMsg{
			Type:    Swap,
			Payload: string(html),
		})
	}

	return errors.New("WebSocket not found")
}

func WebSocketEvent(socketId, event string) error {
	if conn, ok := socketConnections[socketId]; ok {
		return websocket.JSON.Send(conn.Conn, WebSocketMsg{
			Type:    Event,
			Payload: string(event),
		})
	}

	return errors.New("WebSocket not found")
}
