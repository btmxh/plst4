package middlewares

import (
	"errors"
	"net/http"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/errs"
	"github.com/btmxh/plst4/internal/services"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
)

var notOwnerError = errors.New("You must be the owner of this playlist to do this.")

func OwnerCheckMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		handler := errs.NewGinErrorHandler(ctx, "Error")
		id := stores.GetPlaylistId(ctx)

		tx := db.BeginTx(handler)
		if tx == nil {
			return
		}
		defer tx.Rollback()

		if isManager, hasErr := services.IsPlaylistManager(tx, stores.GetUsername(ctx), id); !isManager || hasErr {
			if !hasErr {
				handler.PublicError(http.StatusForbidden, notManagerError)
			}
			return
		}

		if tx.Commit() {
			return
		}

		ctx.Next()
	}
}
