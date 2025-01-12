package middlewares

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/errs"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
)

var InvalidPlaylistIdError = errors.New("Invalid playlist ID.")

func PlaylistIdMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		handler := errs.NewGinErrorHandler(ctx, "Error")
		id := ctx.Param("id")

		if id == "" {
			handler.PublicError(http.StatusUnprocessableEntity, InvalidPlaylistIdError)
			ctx.Abort()
			return
		}

		idInt, err := strconv.Atoi(id)
		if err != nil {
			handler.PrivateError(err)
			handler.PublicError(http.StatusUnprocessableEntity, InvalidPlaylistIdError)
			ctx.Abort()
			return
		}

		stores.SetPlaylistId(ctx, idInt)

		tx := db.BeginTx(handler)
		if tx == nil {
			return
		}
		defer tx.Rollback()

		var dummy int
		var hasRow bool
		if tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1", idInt).Scan(&hasRow, &dummy) {
			ctx.Abort()
			return
		}

		if !hasRow {
			handler.PublicError(http.StatusNotFound, InvalidPlaylistIdError)
			ctx.Abort()
			return
		}

		if tx.Commit() {
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}
