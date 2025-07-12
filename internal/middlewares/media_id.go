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

var InvalidMediaIdError = errors.New("Invalid media ID.")

func MediaIdMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		handler := errs.NewGinErrorHandler(ctx, "Error")
		id := ctx.Param("id")

		if id == "" {
			handler.PublicError(http.StatusUnprocessableEntity, InvalidMediaIdError)
			ctx.Abort()
			return
		}

		idInt, err := strconv.Atoi(id)
		if err != nil {
			handler.PrivateError(err)
			handler.PublicError(http.StatusUnprocessableEntity, InvalidMediaIdError)
			ctx.Abort()
			return
		}

		stores.SetMediaId(ctx, idInt)

		tx := db.BeginTx(handler)
		if tx == nil {
			return
		}
		defer tx.Rollback()

		var dummy int
		var hasRow bool
		if tx.QueryRow("SELECT 1 FROM medias WHERE id = $1", idInt).Scan(&hasRow, &dummy) {
			ctx.Abort()
			return
		}

		if !hasRow {
			handler.PublicError(http.StatusNotFound, InvalidMediaIdError)
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
