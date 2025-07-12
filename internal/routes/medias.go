package routes

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/errs"
	"github.com/btmxh/plst4/internal/html"
	"github.com/btmxh/plst4/internal/media"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/btmxh/plst4/internal/services"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
)

var ErrMissingItemId = errors.New("Missing itemId")

func MediasRouter(g *gin.RouterGroup) {
	idGroup := g.Group("/:id/")
	idGroup.Use(ToastErrorMiddleware())
	idGroup.Use(middlewares.MediaIdMiddleware())

	idGroup.POST("update", updateMediaMetadataHandler)
}

func updateMediaMetadataHandler(ctx *gin.Context) {
	handler := errs.NewGinErrorHandler(ctx, "Update error")
	id := stores.GetMediaId(ctx)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	// get media URL from db
	url, hasRow, hasErr := services.GetMediaUrl(tx, id)
	if !hasRow {
		handler.PublicError(http.StatusNotFound, media.ErrMediaNotFound)
		return
	} else if hasErr {
		return
	}

	object, err := media.ProcessURL(url)
	if err != nil {
		handler.PublicError(http.StatusUnprocessableEntity, err)
		return
	}

	canonMedia, err := object.Canonicalize(ctx.Request.Context())
	if err != nil {
		handler.PublicError(http.StatusUnprocessableEntity, err)
		return
	}

	resolvedMedia, err := canonMedia.Resolve(ctx.Request.Context())
	if err != nil {
		handler.PublicError(http.StatusUnprocessableEntity, err)
		return
	}

	resolvedMediaSingle, ok := resolvedMedia.(media.ResolvedMediaObjectSingle)
	if !ok {
		handler.PublicError(http.StatusUnprocessableEntity, errors.New("Media no longer a single media entry"))
	}

	hasErr = services.UpdateMedia(tx, id, resolvedMediaSingle)
	if hasErr {
		handler.PublicError(http.StatusInternalServerError, errors.New("Failed to update media"))
		return
	}
	tx.Commit()

	// announce playlist refresh to all playlists with medias of id `id`
	go func() {
		handler := errs.NewLogErrorHandler("Announce playlist refresh error", func(err error) error { return nil })

		tx := db.BeginTx(handler)
		if tx == nil {
			return
		}
		defer tx.Rollback()

		// write SQL query to query playlists containing playlist items
		// with the media id = id
		var rows *sql.Rows
		if tx.Query(&rows, "SELECT DISTINCT playlist FROM playlist_items WHERE media = $1", id) {
		}

		for rows.Next() {
			var id int
			err := rows.Scan(&id)
			if err != nil {
				handler.PrivateError(err)
			}

			services.WebSocketPlaylistEvent(id, services.PlaylistChanged)
		}

		tx.Commit()
	}()

	Toast(ctx, html.ToastInfo, "Media metadata updated", template.HTML(template.HTMLEscapeString(fmt.Sprintf("Metadata of media at URL '%s' updated.", url))))
}
