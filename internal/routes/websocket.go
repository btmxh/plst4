package routes

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/services"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

func WebSocketRouter(g *gin.RouterGroup) {
	g.GET("/:id", func(c *gin.Context) {
		playlist, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		username := stores.GetUsername(c)
		websocket.Handler(func(conn *websocket.Conn) {
			defer conn.Close()

			socketId := services.GetManager().Add(conn, playlist, username)
			defer services.GetManager().Remove(playlist, username, socketId)

			handler := services.NewWebSocketErrorHandler("WebSocket error", socketId)
			tx := db.BeginTx(handler)
			if tx == nil {
				return
			}
			defer tx.Rollback()

			if services.NotifyMediaChanged(tx, playlist, socketId) {
				return
			}

			for {
				var msg services.WebSocketMsg
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
