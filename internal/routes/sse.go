package routes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/services"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
)

func SSERouter(g *gin.RouterGroup) {
	g.GET("/:id", func(c *gin.Context) {
		playlist, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)

		messages := make(chan services.WebSocketMsg, 16)
		client := services.RealtimeClientFunc(func(msg services.WebSocketMsg) error {
			select {
			case messages <- msg:
				return nil
			case <-c.Request.Context().Done():
				return c.Request.Context().Err()
			}
		})

		username := stores.GetUsername(c)
		socketId := services.GetManager().AddClient(client, playlist, username)
		defer services.GetManager().Remove(playlist, username, socketId)

		handler := services.NewWebSocketErrorHandler("SSE error", socketId)
		tx := db.BeginTx(handler)
		if tx == nil {
			return
		}
		defer tx.Rollback()

		callback, hasErr := services.NotifyMediaChanged(tx, playlist, socketId)
		if hasErr {
			return
		}

		if tx.Commit() {
			return
		}

		callback()
		flusher.Flush()

		for {
			select {
			case <-c.Request.Context().Done():
				return
			case msg := <-messages:
				data, err := json.Marshal(msg)
				if err != nil {
					slog.Warn("Unable to encode SSE message", "sid", socketId, "msg", msg, "err", err)
					continue
				}

				if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
					slog.Info("SSE connection closed or error", "err", err)
					return
				}
				flusher.Flush()
			}
		}
	})
}
