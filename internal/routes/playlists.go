package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/errs"
	"github.com/btmxh/plst4/internal/html"
	"github.com/btmxh/plst4/internal/media"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/btmxh/plst4/internal/services"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
)

var watchTemplate = getTemplate("watch", "templates/watch.tmpl")
var playlistWatchTmpl = getTemplate("watch", "templates/playlists/watch.tmpl")
var playlistWatchInvalidTmpl = getTemplate("watch", "templates/playlists/watch_invalid.tmpl")
var playlistQueryResult = getTemplate("playlist_query_result", "templates/playlists/playlist_query_result.tmpl")

var playlistNameRegex = regexp.MustCompile(`^.{4,100}$`)
var noCurrentMediaError = errors.New("No currently playing media.")
var invalidItemError = errors.New("Invalid playlist item ID.")
var invalidFormData = errors.New("Invalid form data.")

func getCheckedItems(c *gin.Context, handler errs.ErrorHandler) (ids []int, hasErr bool) {
	var args map[string][]string
	if c.Request.Method == "DELETE" {
		args = c.Request.URL.Query()
	} else {
		if err := c.Request.ParseForm(); err != nil {
			handler.PrivateError(err)
			handler.PublicError(http.StatusUnprocessableEntity, invalidFormData)
			return nil, true
		}
		args = c.Request.PostForm
	}

	var items []int
	for key, value := range args {
		if !slices.Contains(value, "on") {
			continue
		}

		idStr := strings.TrimPrefix(key, "pic-")
		if idStr == key {
			continue
		}

		id, err := strconv.Atoi(idStr)
		if err != nil {
			slog.Warn("Invalid playlist item ID (ignored)", "id", idStr, "err", err)
			continue
		}

		items = append(items, id)

	}

	return items, false
}

func WatchRouter(g *gin.RouterGroup) {
	g.GET("/", html.RenderGinFunc(watchTemplate, "layout", gin.H{}))

	watchPageRouter := g.Group("/:id/")
	watchPageRouter.Use(RenderErrorMiddleware())
	watchPageRouter.Use(middlewares.PlaylistIdMiddleware())
	watchPageRouter.GET("/", playlistWatch)

	idGroup := g.Group("/:id/")
	idGroup.Use(ToastErrorMiddleware())
	idGroup.Use(middlewares.PlaylistIdMiddleware())

	loggedInGroup := idGroup.Group("")
	loggedInGroup.Use(middlewares.MustAuthMiddleware())

	managerGroup := loggedInGroup.Group("")
	managerGroup.Use(middlewares.ManagerCheckMiddleware())

	ownerGroup := loggedInGroup.Group("")
	ownerGroup.Use(middlewares.OwnerCheckMiddleware())

	idGroup.GET("/controller", playlistWatchController)
	managerGroup.POST("/controller/submit", playlistSubmitMetadata)
	ownerGroup.PATCH("/controller/rename", func(c *gin.Context) {
		if name, hasErr := playlistRenameCommon(c); !hasErr {
			UpdateTitle(c, fmt.Sprintf("plst4 - %s", name))
		}
	})
	ownerGroup.DELETE("/controller/delete", func(c *gin.Context) {
		if !playlistDeleteCommon(c) {
			HxRedirect(c, "/watch")
		}
	})
	idGroup.GET("/queue", playlistWatchQueue)
	idGroup.GET("/queue/current", playlistWatchQueueCurrent)
	managerGroup.POST("/queue/add", playlistAdd)
	managerGroup.DELETE("/queue/delete", playlistItemsDelete)
	managerGroup.PATCH("/queue/goto/:item-id", playlistGoto)
	loggedInGroup.POST("/queue/nextreq", playlistNextRequest)
	managerGroup.POST("/queue/prev", playlistPrev)
	managerGroup.POST("/queue/next", playlistNext)
	managerGroup.POST("/queue/up", playlistMoveUp)
	managerGroup.POST("/queue/down", playlistMoveDown)
	idGroup.GET("/managers", playlistManagers)
	ownerGroup.POST("/managers/add", playlistManagerAdd)
	ownerGroup.DELETE("/managers/delete", playlistManagerDelete)

}

func PlaylistRouter(g *gin.RouterGroup) {
	g.GET("/search", search)
	mustAuth := g.Group("")
	mustAuth.Use(ToastErrorMiddleware())
	mustAuth.Use(middlewares.MustAuthMiddleware())
	mustAuth.POST("/new", newPlaylist)

	idGroup := mustAuth.Group("/:id/")
	idGroup.Use(middlewares.PlaylistIdMiddleware())
	idGroup.PATCH("/rename", func(c *gin.Context) {
		if _, hasErr := playlistRenameCommon(c); !hasErr {
			HxRefresh(c)
		}
	})
	idGroup.DELETE("/delete", func(c *gin.Context) {
		if !playlistDeleteCommon(c) {
			HxRefresh(c)
		}
	})
}

var invalidOffsetError = errors.New("Invalid offset.")
var invalidTitleError = errors.New("Invalid title.")
var invalidPageError = errors.New("Invalid page.")

func search(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist search error")
	username := stores.GetUsername(c)

	query := strings.ToLower(c.Query("query"))

	filter, err := services.ParsePlaylistFilter(c.Query("filter"))
	if err != nil {
		handler.PublicError(http.StatusUnprocessableEntity, err)
		return
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		handler.PrivateError(err)
		handler.PublicError(http.StatusUnprocessableEntity, invalidOffsetError)
		return
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	playlists, hasErr := services.SearchPlaylists(tx, username, query, filter, offset)
	if hasErr {
		return
	}

	if tx.Commit() {
		return
	}

	args := gin.H{}
	url := *c.Request.URL
	if playlists.PrevOffset >= 0 {
		query := url.Query()
		query.Set("offset", strconv.Itoa(playlists.PrevOffset))
		url.RawQuery = query.Encode()
		args["PrevURL"] = url.String()
	}

	if playlists.NextOffset >= 0 {
		query := url.Query()
		query.Set("offset", strconv.Itoa(playlists.NextOffset))
		url.RawQuery = query.Encode()
		args["NextURL"] = url.String()
	}

	args["Results"] = playlists.Items
	args["Page"] = playlists.Page
	html.RenderGin(playlistQueryResult, c, "content", args)
}

func newPlaylist(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "New playlist error")
	name, err := HxPrompt(c)
	if err != nil || !playlistNameRegex.MatchString(name) {
		if err != nil {
			handler.PrivateError(err)
		}
		handler.PublicError(http.StatusUnprocessableEntity, invalidTitleError)
		return
	}

	username := stores.GetUsername(c)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	id, hasErr := services.CreatePlaylist(tx, username, name)
	if hasErr {
		return
	}

	if tx.Commit() {
		return
	}

	HxRedirect(c, "/watch/"+strconv.Itoa(id))
}

func playlistRenameCommon(c *gin.Context) (title string, hasErr bool) {
	handler := errs.NewGinErrorHandler(c, "Playlist rename error")
	username := stores.GetUsername(c)
	id := stores.GetPlaylistId(c)

	name, err := HxPrompt(c)
	if err != nil || !playlistNameRegex.MatchString(name) {
		if err != nil {
			handler.PrivateError(err)
		}

		handler.PublicError(http.StatusUnprocessableEntity, invalidTitleError)
		return "", true
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return "", true
	}
	defer tx.Rollback()

	if services.RenamePlaylist(tx, username, id, name) {
		return "", true
	}

	if tx.Commit() {
		return "", true
	}

	services.WebSocketPlaylistEvent(id, services.PlaylistChanged)
	return name, false
}

func playlistDeleteCommon(c *gin.Context) (hasErr bool) {
	handler := errs.NewGinErrorHandler(c, "Playlist delete error")
	username := stores.GetUsername(c)
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	if services.DeletePlaylist(tx, username, id) {
		return
	}

	return tx.Commit()
}

func playlistWatch(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist watch error")
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var name string
	var hasRow bool
	if tx.QueryRow("SELECT name FROM playlists WHERE id = $1", id).Scan(&hasRow, &name) {
		return
	}

	if tx.Commit() {
		return
	}

	html.RenderGin(playlistWatchTmpl, c, "layout", gin.H{
		"Id":    id,
		"Title": name,
	})
}

func playlistRenderQueue(c *gin.Context, playlist int, pageNum int) {
	handler := errs.NewGinErrorHandler(c, "Playlist render queue error")
	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	isManager, hasErr := services.IsPlaylistManager(tx, stores.GetUsername(c), playlist)
	if hasErr {
		return
	}

	var current sql.NullInt32
	var owner string
	if tx.QueryRow("SELECT current, owner_username FROM playlists WHERE id = $1", playlist).Scan(nil, &current, &owner) {
		return
	}

	page, hasErr := services.EnumeratePlaylistItems(tx, playlist, pageNum)
	if hasErr {
		return
	}

	if tx.Commit() {
		return
	}

	currentId := 0
	if current.Valid {
		currentId = int(current.Int32)
	}
	slices.Reverse(page.Items)
	args := gin.H{
		"Id":        playlist,
		"Items":     page.Items,
		"ThisPage":  page.Page,
		"Current":   currentId,
		"Owner":     owner,
		"IsManager": isManager,
	}

	if page.NextOffset >= 0 {
		args["NextPage"] = page.NextPage
	}
	if page.PrevOffset >= 0 {
		args["PrevPage"] = page.PrevPage
	}

	html.RenderGin(playlistWatchTmpl, c, "queue", args)
}

func playlistWatchQueue(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist watch queue error")
	id := stores.GetPlaylistId(c)

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		handler.PrivateError(err)
		handler.PublicError(http.StatusUnprocessableEntity, invalidPageError)
		return
	}

	playlistRenderQueue(c, id, page)
}

func playlistWatchQueueCurrent(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist watch queue current error")
	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	id := stores.GetPlaylistId(c)

	var current sql.NullInt32
	if tx.QueryRow("SELECT current FROM playlists WHERE id = $1", id).Scan(nil, &current) {
		return
	}

	if !current.Valid {
		handler.PublicError(http.StatusNotFound, noCurrentMediaError)
		return
	}

	var index int
	if tx.QueryRow("SELECT COALESCE(COUNT(*), 0) FROM playlist_items WHERE item_order < (SELECT item_order FROM playlist_items WHERE id = $1 AND playlist = $2) AND playlist = $2", current, id).Scan(nil, &index) {
		return
	}

	if tx.Commit() {
		return
	}

	playlistRenderQueue(c, id, 1+index/services.DefaultPagingLimit)
}

func playlistWatchController(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist watch controller error")
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var name string
	var owner string
	var createdTimestamp time.Time
	var current sql.NullInt32
	var hasRow bool
	if tx.QueryRow("SELECT name, owner_username, created_timestamp, current FROM playlists WHERE id = $1", id).Scan(&hasRow, &name, &owner, &createdTimestamp, &current) {
		return
	}

	username := stores.GetUsername(c)
	isManager, hasErr := services.IsPlaylistManager(tx, username, id)
	if hasErr {
		return
	}

	args := gin.H{
		"Id":               id,
		"Name":             name,
		"Owner":            owner,
		"CreatedTimestamp": createdTimestamp,
		"IsManager":        isManager,
	}

	if current.Valid {
		var mediaId int
		var mediaType string
		var title string
		var artist string
		var altTitle string
		var altArtist string
		var duration int
		var url string
		var mediaAddTimestamp time.Time
		var itemAddTimestamp time.Time
		if tx.QueryRow(`
			SELECT
				m.id,
				m.media_type,
				m.title,
				m.artist,
				COALESCE(a.alt_title, m.title),
				COALESCE(a.alt_artist, m.artist),
				m.duration,
				m.url,
				m.add_timestamp,
				i.add_timestamp
			FROM playlist_items i
			JOIN medias m ON i.media = m.id
			LEFT JOIN alt_metadata a ON a.media = m.id AND a.playlist = i.playlist
			WHERE i.id = $1`, current).Scan(nil, &mediaId, &mediaType, &title, &artist, &altTitle, &altArtist, &duration, &url, &mediaAddTimestamp, &itemAddTimestamp) {
			return
		}
		args["Media"] = gin.H{
			"Id":                mediaId,
			"ItemId":            current.Int32,
			"Type":              mediaType,
			"URL":               url,
			"Title":             altTitle,
			"Artist":            altArtist,
			"ThumbnailUrl":      media.GetThumbnail(sql.NullString{String: url, Valid: true}),
			"OriginalTitle":     title,
			"OriginalArtist":    artist,
			"Duration":          time.Duration(duration) * time.Second,
			"MediaAddTimestamp": mediaAddTimestamp,
			"ItemAddTimestamp":  itemAddTimestamp,
		}
	}

	if tx.Commit() {
		return
	}

	html.RenderGin(playlistWatchTmpl, c, "controller", args)
}

func playlistResolveAndAdd(ctx context.Context, handler errs.ErrorHandler, playlist int, mediaObj media.MediaObject, pos services.PlaylistAddPosition) (msg template.HTML, hasErr bool) {
	isSingle := true
	canonMedia, err := mediaObj.Canonicalize(ctx)
	if err != nil {
		handler.PublicError(http.StatusUnprocessableEntity, err)
		return msg, true
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var resolvedMedia media.ResolvedMediaObject
	resolvedMedia, hasRow, hasErr := services.GetResolvedMedia(tx, canonMedia.URL().String())
	var mediaIds []int
	if hasErr {
		return msg, true
	}

	if !hasRow {
		resolvedMedia, err = canonMedia.Resolve(ctx)
		if err != nil {
			handler.PublicError(http.StatusUnprocessableEntity, err)
			return msg, true
		}

		var resolvedMediaSingle media.ResolvedMediaObjectSingle
		resolvedMediaSingle, isSingle = resolvedMedia.(media.ResolvedMediaObjectSingle)
		if isSingle {
			if id, hasErr := services.AddMedia(tx, resolvedMediaSingle); !hasErr {
				mediaIds = append(mediaIds, id)
			} else {
				return msg, true
			}
		} else {
			for _, media := range resolvedMedia.ChildEntries() {
				if id, hasErr := services.AddMedia(tx, media); !hasErr {
					mediaIds = append(mediaIds, id)
				} else {
					return msg, true
				}
			}
		}
	} else {
		id, hasRow, hasErr := services.GetMediaId(tx, canonMedia.URL().String())
		if hasErr {
			return msg, true
		}

		if !hasRow {
			panic("should not happen")
		}

		mediaIds = []int{id}
	}

	var current sql.NullInt32
	if pos == services.QueueNext {
		if tx.QueryRow("SELECT current FROM playlists WHERE id = $1", playlist).Scan(nil, &current) {
			return
		}
		if !current.Valid {
			pos = services.AddToEnd
		}
	}

	var begin int
	delta := services.PlaylistAddOrderGap
	if pos != services.QueueNext {
		var minOrder int
		var maxOrder int
		if tx.QueryRow("SELECT COALESCE(MIN(item_order), 0), COALESCE(MAX(item_order), 0) FROM playlist_items WHERE playlist = $1", playlist).Scan(&hasRow, &minOrder, &maxOrder) {
			return
		}

		if hasRow {
			if pos == services.AddToStart {
				begin = minOrder - delta*len(mediaIds)
			} else {
				begin = maxOrder + delta
			}
		} else {
			begin = 0
		}
	} else {
		prev := int(current.Int32)
		var prevOrder, nextOrder int
		prevOrder, hasErr = services.GetPlaylistItemOrder(tx, prev)
		if hasErr {
			return msg, true
		}

		slog.Info("Adding playlist item between...", "prev", prev, "prev_order", prevOrder)
		var next int
		next, nextOrder, hasRow, hasErr = services.GetNextPlaylistItem(tx, playlist, prevOrder)
		if hasErr {
			return msg, true
		}

		if !hasRow {
			slog.Info("and nothing!")
			delta = services.PlaylistAddOrderGap
			begin = prevOrder + delta
		} else {
			slog.Info("and...", "next", next, "next_order", nextOrder)
			slog.Info("local rebalancing", "playlist", playlist, "prev_order", prevOrder, "next_order", nextOrder, "prev", prev, "next", next, "len", len(mediaIds))
			begin, delta, hasErr = services.LocalRebalance(tx, playlist, prevOrder, nextOrder, prev, len(mediaIds))
			if hasErr {
				return msg, true
			}
		}
	}

	slog.Info("Adding media to playlist", "playlist", playlist, "mediaIds", mediaIds, "begin", begin, "delta", delta)
	if _, hasErr = services.AddPlaylistItems(tx, playlist, mediaIds, begin, delta); hasErr {
		return
	}

	if tx.Commit() {
		return
	}

	if isSingle {
		msg = html.StringAsHTML(fmt.Sprintf("Media list %s - %s added to playlist", resolvedMedia.Title(), resolvedMedia.Artist()))
	} else {
		msg = html.StringAsHTML(fmt.Sprintf("Media %s - %s added to playlist", resolvedMedia.Title(), resolvedMedia.Artist()))
	}

	services.WebSocketPlaylistEvent(playlist, services.PlaylistChanged)
	return msg, false
}

func playlistAdd(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist add error")
	id := stores.GetPlaylistId(c)

	pos, err := services.ParsePlaylistAddPosition(c.PostForm("position"))
	if err != nil {
		handler.PublicError(http.StatusUnprocessableEntity, err)
		return
	}

	url := c.PostForm("url")
	canonInfo, err := media.ProcessURL(url)
	if err != nil {
		handler.PublicError(http.StatusUnprocessableEntity, err)
		return
	}

	wsId := c.PostForm("websocket-id")
	if wsId == "" {
		msg, hasErr := playlistResolveAndAdd(c.Request.Context(), handler, id, canonInfo, pos)
		if hasErr {
			return
		}

		Toast(c, html.ToastInfo, "Media added successfully", msg)
	} else {
		go func() {
			msg, hasErr := playlistResolveAndAdd(context.Background(), services.NewWebSocketErrorHandler("Unable to add media to playlist", wsId), id, canonInfo, pos)
			if hasErr {
				return
			}

			services.WebSocketToast(wsId, html.ToastInfo, "Media added successfully", msg)
		}()
		Toast(c, html.ToastInfo, "Adding new media", template.HTML(template.HTMLEscapeString(fmt.Sprintf("Adding media with URL %s to playlist...", url))))
	}
}

func playlistGoto(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist goto error")
	id := stores.GetPlaylistId(c)

	itemId, err := strconv.Atoi(c.Param("item-id"))
	if err != nil {
		handler.PrivateError(err)
		handler.PublicError(http.StatusNotFound, invalidItemError)
		return
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	if hasItem, hasErr := services.CheckPlaylistItemExists(tx, id, itemId); hasErr || !hasItem {
		if !hasItem {
			handler.PublicError(http.StatusNotFound, invalidItemError)
		}
		return
	}

	if services.SetCurrentMedia(tx, id, sql.NullInt32{Int32: int32(itemId), Valid: true}) {
		return
	}

	callback, hasErr := services.NotifyMediaChanged(tx, id, "")
	if hasErr {
		return
	}

	if tx.Commit() {
		return
	}

	callback()
}

func playlistItemsDelete(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist item delete error")
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	items, hasErr := getCheckedItems(c, handler)
	if hasErr {
		return
	}

	current, hasErr := services.GetCurrentMedia(tx, id)
	if hasErr {
		return
	}

	currentMediaChanged := false
	for _, item := range items {
		if current.Valid && int(current.Int32) == item {
			if services.SetCurrentMedia(tx, id, sql.NullInt32{}) {
				return
			}

			current.Valid = false
			currentMediaChanged = true
		}

		if services.DeletePlaylistItem(tx, id, item) {
			return
		}
	}

	callback := func() {
		services.WebSocketPlaylistEvent(id, services.PlaylistChanged)
	}
	if currentMediaChanged {
		callback, hasErr = services.NotifyMediaChanged(tx, id, "")
		if hasErr {
			return
		}
	}

	if tx.Commit() {
		return
	}

	callback()
	Toast(c, html.ToastInfo, "Playlist items removed", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d item(s) are removed from the playlist", len(items)))))
}

func playlistSubmitMetadata(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist metadata submit error")
	id := stores.GetPlaylistId(c)
	title := c.PostForm("media-title")
	artist := c.PostForm("media-artist")

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var media int
	var hasRow bool
	if tx.QueryRow("SELECT i.media FROM playlists p JOIN playlist_items i ON p.current = i.id WHERE p.id = $1", id).Scan(&hasRow, &media) {
		return
	}

	if !hasRow {
		handler.PublicError(http.StatusNotFound, noCurrentMediaError)
		return
	}

	if services.SetMediaAltMetadata(tx, title, artist, id, media) {
		return
	}

	if tx.Commit() {
		return
	}

	services.WebSocketPlaylistEvent(id, services.PlaylistChanged)
	Toast(c, html.ToastInfo, "Metadata updated", "Metadata of current playlist item was updated successfully")
}

func playlistNextRequest(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist next request error")
	quiet := c.PostForm("quiet") == "true"
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	callback, hasErr := services.SendNextRequest(tx, handler, id, stores.GetUsername(c))
	if hasErr {
		return
	}

	if !quiet {
		Toast(c, html.ToastInfo, "Next request sent", "Successfully sent next request")
	}

	if tx.Commit() {
		return
	}

	callback()
}

func playlistPrev(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist prev error")
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	callback, hasErr := services.PlaylistUpdateCurrent(tx, handler, id, "<", "DESC")
	if hasErr {
		return
	}

	tx.Commit()

	callback()
}

func playlistNext(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist next error")
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	callback, hasErr := services.PlaylistUpdateCurrent(tx, handler, id, ">", "ASC")
	if hasErr {
		return
	}

	tx.Commit()

	callback()
}

func playlistManagers(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist managers error")
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	owner, hasErr := services.GetPlaylistOwner(tx, id)
	if hasErr {
		return
	}

	managers, hasErr := services.EnumeratePlaylistManagers(tx, id)
	if hasErr {
		return
	}

	if tx.Commit() {
		return
	}

	html.RenderGin(playlistWatchTmpl, c, "managers", gin.H{
		"Id":       id,
		"Owner":    owner,
		"Managers": managers,
	})
}

func playlistManagerAdd(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist manager add error")
	id := stores.GetPlaylistId(c)

	username, err := HxPrompt(c)
	if err != nil {
		handler.PublicError(http.StatusUnprocessableEntity, err)
		return
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	if services.AddPlaylistManager(tx, id, username) {
		return
	}

	if tx.Commit() {
		return
	}

	services.WebSocketPlaylistEvent(id, services.ManagersChanged)
	Toast(c, html.ToastInfo, "New manager successfully added", template.HTML(template.HTMLEscapeString(fmt.Sprintf("User '%s' is now a manager of the playlist", username))))
}

func playlistManagerDelete(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist manager delete error")
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var numAffected int
	for manager, value := range c.Request.URL.Query() {
		if !slices.Contains(value, "on") {
			continue
		}

		if services.DeletePlaylistManager(tx, id, manager) {
			return
		}
	}

	if tx.Commit() {
		return
	}

	services.WebSocketPlaylistEvent(id, services.ManagersChanged)
	Toast(c, html.ToastInfo, "Managers successfully removed", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d manager(s) are removed from playlist", numAffected))))
}

func playlistMoveUp(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist move up error")
	id := stores.GetPlaylistId(c)

	items, hasErr := getCheckedItems(c, handler)
	if hasErr {
		return
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	numAffected, hasErr := services.MoveItems(tx, id, items, services.MoveUp)
	if hasErr {
		return
	}

	if tx.Commit() {
		return
	}

	services.WebSocketPlaylistEvent(id, services.PlaylistChanged)
	Toast(c, html.ToastInfo, "Playlist items reordered", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d item(s) affected", numAffected))))
}

func playlistMoveDown(c *gin.Context) {
	handler := errs.NewGinErrorHandler(c, "Playlist move down error")
	id := stores.GetPlaylistId(c)

	items, hasErr := getCheckedItems(c, handler)
	if hasErr {
		return
	}

	tx := db.BeginTx(handler)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	numAffected, hasErr := services.MoveItems(tx, id, items, services.MoveDown)
	if hasErr {
		return
	}

	if tx.Commit() {
		return
	}

	services.WebSocketPlaylistEvent(id, services.PlaylistChanged)
	Toast(c, html.ToastInfo, "Playlist items reordered", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d item(s) affected", numAffected))))
}
