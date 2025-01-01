package middlewares

import (
	"errors"
	"strconv"

	"github.com/btmxh/plst4/internal/errs"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
)

var invalidPlaylistIdError = errors.New("Invalid playlist ID")

func PlaylistIdMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id := ctx.Param("id")
		if id == "" {
			errs.PublicError(ctx, invalidPlaylistIdError)
			return
		}

		idInt, err := strconv.Atoi(id)
		if err != nil {
			errs.PrivateError(ctx, err)
			errs.PublicError(ctx, invalidPlaylistIdError)
			return
		}

		stores.SetPlaylistId(ctx, idInt)
		ctx.Next()
	}
}
