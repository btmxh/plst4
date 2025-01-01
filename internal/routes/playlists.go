package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/btmxh/plst4/internal/auth"
	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/errs"
	"github.com/btmxh/plst4/internal/html"
	"github.com/btmxh/plst4/internal/media"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/btmxh/plst4/internal/stores"
	"github.com/gin-gonic/gin"
)

type PlaylistFilter string
type PlaylistAddPosition string

const (
	AddToStart PlaylistAddPosition = "add-to-start"
	AddToEnd   PlaylistAddPosition = "add-to-end"
	QueueNext  PlaylistAddPosition = "queue-next"
	gap        int                 = 1 << 10
)

type QueriedPlaylist struct {
	Id               int
	Name             string
	OwnerUsername    string
	CreatedTimestamp time.Time
	ItemCount        int
	TotalLength      time.Duration
	CurrentPlaying   string
}

type QueuePlaylistItem struct {
	Title    string
	Artist   string
	URL      string
	Duration time.Duration
	Id       int
	Index    int
}

const (
	All     PlaylistFilter = "all"
	Owned   PlaylistFilter = "owned"
	Managed PlaylistFilter = "managed"
)

var watchTemplate = getTemplate("watch", "templates/watch.tmpl")
var playlistWatchTmpl = getTemplate("watch", "templates/playlists/watch.tmpl")
var playlistWatchInvalidTmpl = getTemplate("watch", "templates/playlists/watch_invalid.tmpl")
var playlistQueryResult = getTemplate("playlist_query_result", "templates/playlists/playlist_query_result.tmpl")

var playlistNameRegex = regexp.MustCompile(`^[a-zA-Z0-9 ]{4,100}$`)
var unauthorizedError = errors.New("You must be logged in to do this.")
var invalidPlaylistError = errors.New("Invalid playlist. Refresh the page maybe?")
var missingPermissionError = errors.New("You must be a manager of this playlist to do this.")
var noCurrentMediaError = errors.New("No currently playing media.")

func noswap(c *gin.Context) {
	c.Header("Hx-Reswap", "none")
}

func isManager(tx *db.Tx, username string, playlist int) (isManager, hasError bool) {
	var dummy int
	var hasRow bool
	if tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1 AND owner_username = $2", playlist, username).Scan(&hasRow, &dummy) {
		return false, true
	}

	if hasRow {
		return true, false
	}

	if tx.QueryRow("SELECT 1 FROM playlist_manage WHERE playlist = $1 AND username = $2", playlist, username).Scan(&hasRow, &dummy) {
		return false, true
	}

	return hasRow, false
}

func getCheckedItems(c *gin.Context) ([]int, error) {
	var args map[string][]string
	if c.Request.Method == "DELETE" {
		args = c.Request.URL.Query()
	} else {
		if err := c.Request.ParseForm(); err != nil {
			return nil, err
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
			slog.Warn("Invalid ID (ignoring): %s (parse error %v)", idStr, err)
			return nil, err
		}

		items = append(items, id)

	}

	return items, nil
}

func updateTitle(c *gin.Context, title string) {
	title = template.HTMLEscapeString(title)
	c.Writer.WriteString("<title>")
	c.Writer.WriteString(title)
	c.Writer.WriteString("</title>")
}

func WatchRouter(g *gin.RouterGroup) {
	g.GET("/", html.RenderFunc(watchTemplate, "layout", gin.H{}))
	idGroup := g.Group("/:id/")
	idGroup.Use(middlewares.PlaylistIdMiddleware())
	idGroup.GET("/", playlistWatch)
	idGroup.GET("/controller", playlistWatchController)
	idGroup.POST("/controller/submit", playlistSubmitMetadata)
	idGroup.PATCH("/controller/rename", func(c *gin.Context) {
		name, renamed := renamePlaylist(c)
		if renamed {
			updateTitle(c, fmt.Sprintf("plst4 - %s", name))
		}
		playlistWatchController(c)
	})
	idGroup.DELETE("/controller/delete", func(c *gin.Context) {
		deletePlaylist(c)
		HxRedirect(c, "/watch")
	})
	idGroup.GET("/queue", playlistWatchQueue)
	idGroup.GET("/queue/current", playlistWatchQueueCurrent)
	idGroup.POST("/queue/add", playlistAdd)
	idGroup.DELETE("/queue/delete", playlistDelete)
	idGroup.PATCH("/queue/goto/:item-id", playlistGoto)
	idGroup.POST("/queue/nextreq", playlistNextRequest)
	idGroup.POST("/queue/prev", playlistPrev)
	idGroup.POST("/queue/next", playlistNext)
	idGroup.POST("/queue/up", playlistMoveUp)
	idGroup.POST("/queue/down", playlistMoveDown)
	idGroup.GET("/managers", playlistManagers)
	idGroup.POST("/managers/add", playlistManagerAdd)
	idGroup.DELETE("/managers/delete", playlistManagerDelete)

}

func PlaylistRouter(g *gin.RouterGroup) {
	g.GET("/search", search)
	g.POST("/new", newPlaylist)
	g.PATCH("/:id/rename", func(c *gin.Context) {
		renamePlaylist(c)
		HxRefresh(c)
	})
	g.DELETE("/:id/delete", func(c *gin.Context) {
		deletePlaylist(c)
		HxRefresh(c)
	})
}

var invalidParam = errors.New("Invalid parameter")

func search(c *gin.Context) {
	username := auth.GetUsername(c)

	query := strings.ToLower(c.Query("query"))
	filter := c.Query("filter")
	offsetStr := c.DefaultQuery("offset", "0")
	limit := 10

	offset, err := strconv.Atoi(offsetStr)

	if err != nil {
		errs.PrivateError(c, fmt.Errorf("Invalid offset: %w", err))
		errs.PublicError(c, invalidParam)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var rows *sql.Rows
	var hasErr bool
	if filter == string(All) {
		hasErr = tx.Query(&rows,
			`SELECT id, name, owner_username, created_timestamp, 
              COALESCE((SELECT COUNT(*) FROM playlist_items WHERE playlist = playlists.id), 0),
							COALESCE((SELECT SUM(m.duration) FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = playlists.id), 0)
         FROM playlists 
         WHERE POSITION($1 IN LOWER(name)) > 0 
         ORDER BY created_timestamp 
         LIMIT $2 OFFSET $3`,
			query, limit+1, offset,
		)
	} else if filter == string(Owned) {
		hasErr = tx.Query(&rows,
			`SELECT id, name, owner_username, created_timestamp, 
              COALESCE((SELECT COUNT(*) FROM playlist_items WHERE playlist = playlists.id), 0),
							COALESCE((SELECT SUM(m.duration) FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = playlists.id), 0)
         FROM playlists 
         WHERE POSITION($1 IN LOWER(name)) > 0 
           AND owner_username = $4 
         ORDER BY created_timestamp 
         LIMIT $2 OFFSET $3`,
			query, limit+1, offset, username,
		)
	} else if filter == string(Managed) {
		hasErr = tx.Query(&rows,
			`SELECT id, name, owner_username, created_timestamp, 
              COALESCE((SELECT COUNT(*) FROM playlist_items WHERE playlist = playlists.id), 0),
							COALESCE((SELECT SUM(m.duration) FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = playlists.id), 0)
         FROM playlists 
         WHERE POSITION($1 IN LOWER(name)) > 0 
           AND (owner_username = $4 OR $4 IN (SELECT username FROM playlist_manage WHERE playlist = playlists.id))
         ORDER BY created_timestamp 
         LIMIT $2 OFFSET $3`,
			query, limit+1, offset, username,
		)
	} else {
		errs.PrivateError(c, fmt.Errorf("Invalid filter: %s", filter))
		errs.PublicError(c, invalidParam)
		return
	}

	if hasErr {
		return
	}

	var playlists []QueriedPlaylist
	for rows.Next() {
		var playlist QueriedPlaylist
		var totalLength int
		err = rows.Scan(&playlist.Id, &playlist.Name, &playlist.OwnerUsername, &playlist.CreatedTimestamp, &playlist.ItemCount, &totalLength)
		if err != nil {
			errs.PrivateError(c, err)
			return
		}

		playlist.TotalLength = time.Duration(totalLength) * time.Second
		playlists = append(playlists, playlist)
	}

	if tx.Commit() {
		return
	}

	args := gin.H{}
	url := *c.Request.URL
	if offset > 0 {
		query := url.Query()
		query.Set("offset", strconv.Itoa(max(offset-limit, 0)))
		url.RawQuery = query.Encode()
		args["PrevURL"] = url.String()
	}

	if len(playlists) > limit {
		playlists = playlists[:limit]
		query := url.Query()
		query.Set("offset", strconv.Itoa(offset+limit))
		url.RawQuery = query.Encode()
		args["NextURL"] = url.String()
	}
	args["Results"] = playlists
	args["Page"] = 1 + offset/limit

	html.Render(playlistQueryResult, c, "content", args)
}

func newPlaylist(c *gin.Context) {
	if !auth.IsLoggedIn(c) {
		errs.PublicError(c, unauthorizedError)
		return
	}

	name, err := HxPrompt(c)
	if err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, invalidParam)
		return
	}

	if playlistNameRegex.MatchString(name) {
		errs.PrivateError(c, fmt.Errorf("Invalid playlist name: %s", name))
		errs.PublicError(c, invalidParam)
		return
	}

	username := auth.GetUsername(c)

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var id int
	if tx.QueryRow("INSERT INTO playlists (name, owner_username) VALUES ($1, $2) RETURNING id", name, username).Scan(nil, &id) {
		return
	}

	if tx.Commit() {
		return
	}

	HxRedirect(c, "/watch/"+strconv.Itoa(id))
}

func renamePlaylist(c *gin.Context) (string, bool) {
	if !auth.IsLoggedIn(c) {
		errs.PublicError(c, unauthorizedError)
		return "", false
	}

	username := auth.GetUsername(c)
	id := stores.GetPlaylistId(c)

	name, err := HxPrompt(c)
	if err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, invalidParam)
		return "", false
	}

	if playlistNameRegex.MatchString(name) {
		errs.PrivateError(c, fmt.Errorf("Invalid playlist name: %s", name))
		errs.PublicError(c, invalidParam)
		return "", false
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return "", false
	}
	defer tx.Rollback()

	if tx.Exec(nil, "UPDATE playlists SET name = $1 WHERE id = $2 AND owner_username = $3", name, id, username) {
		return "", false
	}

	if tx.Commit() {
		return "", false
	}

	return name, true
}

func deletePlaylist(c *gin.Context) bool {
	if !auth.IsLoggedIn(c) {
		errs.PublicError(c, unauthorizedError)
		return false
	}

	username := auth.GetUsername(c)
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(c)
	if tx == nil {
		return false
	}
	defer tx.Rollback()

	if tx.Exec(nil, "DELETE FROM playlists WHERE id = $1 AND owner_username = $2", id, username) {
		return false
	}

	if tx.Commit() {
		return false
	}

	return true
}

func playlistWatch(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var name string
	var hasRow bool
	if tx.QueryRow("SELECT name FROM playlists WHERE id = $1", id).Scan(&hasRow, &name) {
		return
	}

	if !hasRow {
		html.Render(playlistWatchInvalidTmpl, c, "layout", gin.H{})
		return
	}

	if tx.Commit() {
		return
	}

	html.Render(playlistWatchTmpl, c, "layout", gin.H{
		"Id":    id,
		"Title": name,
	})
}

func playlistRenderQueue(c *gin.Context, playlist int, page int) {
	slog.Warn("Rendering playlist queue", "playlist", playlist, "page", page)
	limit := 10
	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	username := auth.GetUsername(c)
	isManager, hasErr := isManager(tx, username, playlist)
	if hasErr {
		return
	}

	var current sql.NullInt32
	var owner string
	var hasRow bool
	if tx.QueryRow("SELECT current, owner_username FROM playlists WHERE id = $1", playlist).Scan(&hasRow, &current, &owner) {
		return
	}

	if !hasRow {
		errs.PublicError(c, invalidPlaylistError)
		return
	}

	var itemCount int
	if tx.QueryRow("SELECT COUNT(*) FROM playlist_items WHERE playlist = $1", playlist).Scan(&hasRow, &itemCount) {
		return
	}

	if page == 0 {
		// last page
		page = (itemCount + limit - 1) / limit
		slog.Warn("page", "p", itemCount)
	}

	offset := (page - 1) * 10

	var items []QueuePlaylistItem
	var rows *sql.Rows
	if tx.Query(&rows, `
    SELECT 
        i.id, 
        COALESCE(a.alt_title, m.title),
        COALESCE(a.alt_artist, m.artist),
        m.url, 
        m.duration 
    FROM playlist_items i 
    JOIN medias m ON m.id = i.media 
		LEFT JOIN alt_metadata a ON a.media = m.id AND a.playlist = i.playlist
    WHERE i.playlist = $1 
    ORDER BY i.item_order 
    OFFSET $2 LIMIT $3`, playlist, offset, limit) {
		return
	}

	for rows.Next() {
		var item QueuePlaylistItem
		var duration time.Duration
		err := rows.Scan(&item.Id, &item.Title, &item.Artist, &item.URL, &duration)
		if err != nil {
			errs.PrivateError(c, err)
			return
		}
		item.Duration = time.Duration(duration) * time.Second
		item.Index = offset
		offset += 1
		items = append(items, item)
	}

	if tx.Commit() {
		return
	}

	currentId := 0
	if current.Valid {
		currentId = int(current.Int32)
	}
	slices.Reverse(items)
	args := gin.H{
		"Id":        playlist,
		"Items":     items,
		"ThisPage":  page,
		"Current":   currentId,
		"Owner":     owner,
		"IsManager": isManager,
	}

	if page > 1 {
		args["PrevPage"] = page - 1
	}
	if itemCount > page*limit {
		args["NextPage"] = page + 1
	}

	html.Render(playlistWatchTmpl, c, "queue", args)
}

func playlistWatchQueue(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, invalidParam)
		return
	}

	playlistRenderQueue(c, id, page)
}

func playlistWatchController(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(c)
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

	username := auth.GetUsername(c)
	isManager, hasErr := isManager(tx, username, id)
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
			WHERE i.id = $1`, current).Scan(nil, &mediaId, &title, &artist, &altTitle, &altArtist, &duration, &url, &mediaAddTimestamp, &itemAddTimestamp) {
			return
		}
		args["Media"] = gin.H{
			"Id":                mediaId,
			"ItemId":            current.Int32,
			"Type":              "yt",
			"URL":               url,
			"Title":             altTitle,
			"Artist":            altArtist,
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

	html.Render(playlistWatchTmpl, c, "controller", args)
}

func localRebalance(tx *sql.Tx, playlist int, startOrder int, endOrder int, numInsert int) error {
	alpha := 1.5
	beta := 2.5

	var count int
	err := tx.QueryRow("SELECT COUNT(*) FROM playlist_items WHERE playlist = $1 AND (item_order BETWEEN $2 AND $3)", playlist, startOrder, endOrder).Scan(&count)
	if err != nil {
		return err
	}

	numOrderValues := endOrder - startOrder + 1
	density := float64(numOrderValues) / float64(count+numInsert)
	if density < beta {
		startOrder = int(float64(startOrder) - alpha*float64(numOrderValues))
		endOrder = int(float64(endOrder) + beta*float64(numOrderValues))
		return localRebalance(tx, playlist, startOrder, endOrder, numInsert)
	}

	_, err = tx.Exec(
		`
		WITH RankedRows AS (
			SELECT
				item_order,
				ROW_NUMBER() OVER (ORDER BY item_order) AS i
			FROM playlist_items
			WHERE playlist = $1 AND item_order BETWEEN $2 AND $3
		), UpdatedRows AS (
			SELECT
				item_order,
				($2 + ($3 - $2) * (i - 1) / $4) AS new_order
			FROM RankedRows
		)
		UPDATE playlist_items
		SET item_order = UpdatedRows.new_order
		FROM UpdatedRows
		WHERE playlist_items.item_order = UpdatedRows.item_order AND playlist = $1
		`, playlist, startOrder, endOrder, count)
	if err != nil {
		return err
	}

	return nil
}

func playlistAddBackground(playlist int, socketId string, canonInfo *media.MediaCanonicalizeInfo, pos PlaylistAddPosition) {
	ctx := context.Background()
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist item add error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}

		WebSocketToast(socketId, ToastError, "Unable to add media to playlist", msg)
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var current sql.NullInt32
	err = tx.QueryRow("SELECT current FROM playlists WHERE id = $1", playlist).Scan(&current)
	if err != nil {
		fail("", err)
		return
	}

	var title string
	var artist string
	var mediaIds []int

	addMedias := func(entries []media.MediaListEntry) error {
		for _, entry := range entries {
			var id int
			err := tx.QueryRow("SELECT id FROM medias WHERE url = $1", entry.CanonInfo.Url).Scan(&id)
			if err != nil && err != sql.ErrNoRows {
				return err
			}

			if err == sql.ErrNoRows {
				slog.Warn(string(entry.CanonInfo.Kind))
				err = tx.QueryRow(
					"INSERT INTO medias (media_type, title, artist, duration, url, aspect_ratio) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
					string(entry.CanonInfo.Kind), entry.ResolveInfo.Title, entry.ResolveInfo.Artist, int(entry.ResolveInfo.Duration.Seconds()), entry.CanonInfo.Url, entry.ResolveInfo.AspectRatio,
				).Scan(&id)
				if err != nil {
					return err
				}
			}

			mediaIds = append(mediaIds, id)
		}

		return nil
	}

	if canonInfo.Multiple {
		mediaList, err := media.ResolveMediaList(ctx, canonInfo)
		if err != nil {
			fail("Unable to resolve media list", err)
			return
		}

		title = mediaList.Title
		artist = mediaList.Artist

		err = addMedias(mediaList.Medias)
		if err != nil {
			fail("Unable to add media to DB", err)
			return
		}
	} else {
		var id int
		err = tx.QueryRow("SELECT id, title, artist FROM medias WHERE url = $1", canonInfo.Url).Scan(&id, &title, &artist)
		if err != nil && err != sql.ErrNoRows {
			fail("", err)
			return
		}

		if err == sql.ErrNoRows {
			resolvedMediaInfo, err := media.ResolveMedia(ctx, canonInfo)
			if err != nil {
				fail("Unable to resolve media", err)
				return
			}

			title = resolvedMediaInfo.Title
			artist = resolvedMediaInfo.Artist

			err = addMedias([]media.MediaListEntry{{CanonInfo: canonInfo, ResolveInfo: resolvedMediaInfo}})
			if err != nil {
				fail("Unable to add media to DB", err)
				return
			}
		} else {
			mediaIds = append(mediaIds, id)
		}
	}
	var itemCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM playlist_items WHERE playlist = $1", playlist).Scan(&itemCount)

	if !current.Valid && pos == QueueNext {
		pos = AddToEnd
	}

	firstOrder := 0
	orderIncrement := gap
	if pos != QueueNext {
		if itemCount > 0 {
			var minOrder int
			var maxOrder int
			err = tx.QueryRow("SELECT MIN(item_order), MAX(item_order) FROM playlist_items WHERE playlist = $1", playlist).Scan(&minOrder, &maxOrder)
			if err != sql.ErrNoRows && err != nil {
				fail("", err)
				return
			}

			if pos == AddToStart {
				firstOrder = minOrder - orderIncrement*len(mediaIds)
			} else {
				firstOrder = maxOrder + orderIncrement
			}
		}
	} else {
		var currentOrder int
		err = tx.QueryRow("SELECT item_order FROM playlist_items WHERE id = $1", current).Scan(&currentOrder)
		if err != nil {
			fail("", err)
			return
		}

		var nextOrder int
		err = tx.QueryRow("SELECT item_order FROM playlist_items WHERE playlist = $1 AND item_order > $2 ORDER BY item_order LIMIT 1", playlist, currentOrder).Scan(&nextOrder)
		if err == sql.ErrNoRows {
			firstOrder = currentOrder + orderIncrement
		} else {
			if err != nil {
				fail("", err)
				return
			}

			if nextOrder <= currentOrder+len(mediaIds) {
				err = localRebalance(tx, playlist, currentOrder, nextOrder, len(mediaIds))
				if err != nil {
					fail("", err)
					return
				}

				err = tx.QueryRow("SELECT item_order FROM playlist_items WHERE id = $1", current).Scan(&currentOrder)
				if err != nil {
					fail("", err)
					return
				}

				err = tx.QueryRow("SELECT item_order FROM playlist_items WHERE playlist = $1 AND item_order > $2 ORDER BY item_order LIMIT 1", playlist, currentOrder).Scan(&nextOrder)
				if err != nil {
					fail("", err)
					return
				}
			}

			orderIncrement = (nextOrder - currentOrder) / (len(mediaIds) + 1)
			firstOrder = currentOrder + orderIncrement
			slog.Warn("haal", "f", firstOrder, "s", currentOrder, "n", nextOrder)
			if orderIncrement == 0 || firstOrder+orderIncrement*(len(mediaIds)-1) == nextOrder {
				panic("playlist too full")
			}
		}
	}

	for i, mediaId := range mediaIds {
		var itemId int
		order := firstOrder + i*orderIncrement
		err = tx.QueryRow("INSERT INTO playlist_items (media, playlist, item_order) VALUES ($1, $2, $3) RETURNING id", mediaId, playlist, order).Scan(&itemId)
		if err != nil {
			fail("", err)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		fail("", err)
		return
	}

	var msg string
	if canonInfo.Multiple {
		msg = fmt.Sprintf("Media %s - %s added to playlist", title, artist)
	} else {
		msg = fmt.Sprintf("Media list %s - %s added to playlist", title, artist)
	}

	WebSocketPlaylistEvent(playlist, PlaylistChanged)
	WebSocketToast(socketId, ToastInfo, "Media added successfully", template.HTML(template.HTMLEscapeString(msg)))
}

func playlistAdd(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	pos := c.PostForm("add-position")
	url := c.PostForm("url")
	wsId := c.PostForm("websocket-id")

	if !auth.IsLoggedIn(c) {
		errs.PublicError(c, unauthorizedError)
		return
	}

	if pos != string(AddToStart) && pos != string(AddToEnd) && pos != string(QueueNext) {
		errs.PrivateError(c, fmt.Errorf("Invalid add position: %s", pos))
		errs.PublicError(c, invalidParam)
		return
	}

	if wsId == "" {
		errs.PrivateError(c, fmt.Errorf("Invalid websocket id: %s", wsId))
		errs.PublicError(c, invalidParam)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var dummy int
	var hasRow bool
	if tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1", id).Scan(&hasRow, &dummy) {
		return
	}

	if !hasRow {
		errs.PublicError(c, invalidPlaylistError)
		return
	}

	username := auth.GetUsername(c)
	isManager, hasErr := isManager(tx, username, id)
	if hasErr {
		return
	}

	if !isManager {
		errs.PublicError(c, missingPermissionError)
		return
	}

	canonInfo, err := media.CanonicalizeMedia(url)
	if err != nil {
		errs.PublicError(c, fmt.Errorf("Invalid media URL: %w", err))
		return
	}

	if tx.Commit() {
		return
	}

	go playlistAddBackground(id, wsId, canonInfo, PlaylistAddPosition(pos))
	Toast(c, ToastInfo, "Adding new media", template.HTML(template.HTMLEscapeString(fmt.Sprintf("Adding media with URL %s to playlist...", url))))
}

func sendMediaChanged(id int, itemId int) {
	var payload MediaChangedPayload
	err := db.DB.QueryRow("SELECT m.media_type, m.url, m.aspect_ratio FROM playlist_items i JOIN medias m ON i.media = m.id WHERE i.id = $1", itemId).Scan(&payload.Type, &payload.Url, &payload.AspectRatio)
	if err != nil && err != sql.ErrNoRows {
		slog.Warn("Unable to query current media", "pid", id, "iid", itemId, "err", err)
		return
	}

	if err == nil {
		WebSocketMediaChange(id, "", payload)
	}
}

func playlistGoto(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	if !auth.IsLoggedIn(c) {
		errs.PublicError(c, unauthorizedError)
		return
	}

	itemId, err := strconv.Atoi(c.Param("item-id"))
	if err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, invalidParam)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	username := auth.GetUsername(c)
	isManager, hasErr := isManager(tx, username, id)
	if hasErr {
		return
	}

	if !isManager {
		errs.PublicError(c, missingPermissionError)
		return
	}

	var dummy int
	var hasRow bool
	if tx.QueryRow("SELECT 1 FROM playlist_items WHERE id = $1 AND playlist = $2", itemId, id).Scan(&hasRow, &dummy) {
		return
	}

	if !hasRow {
		errs.PublicError(c, invalidParam)
		return
	}

	if tx.Exec(nil, "UPDATE playlists SET current = $1 WHERE id = $2", itemId, id) {
		return
	}

	if tx.Commit() {
		return
	}

	go sendMediaChanged(id, itemId)
}

func playlistDelete(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	if !auth.IsLoggedIn(c) {
		errs.PublicError(c, unauthorizedError)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var dummy int
	var hasRow bool
	if tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1", id).Scan(&hasRow, &dummy) {
		return
	}

	if !hasRow {
		errs.PublicError(c, invalidPlaylistError)
		return
	}

	username := auth.GetUsername(c)
	isManager, hasErr := isManager(tx, username, id)
	if hasErr {
		return
	}

	if !isManager {
		errs.PublicError(c, missingPermissionError)
		return
	}

	items, err := getCheckedItems(c)
	if err != nil {
		errs.PrivateError(c, err)
		errs.PublicError(c, invalidParam)
		return
	}

	for _, item := range items {
		if tx.Exec(nil, "DELETE FROM playlist_items WHERE id = $1 AND playlist = $2", item, id) {
			return
		}
	}

	if tx.Commit() {
		return
	}

	WebSocketPlaylistEvent(id, PlaylistChanged)
	Toast(c, ToastInfo, "Playlist items removed", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d item(s) are removed from the playlist", len(items)))))
}

func playlistSubmitMetadata(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	if !auth.IsLoggedIn(c) {
		errs.PublicError(c, unauthorizedError)
		return
	}

	title := c.PostForm("media-title")
	artist := c.PostForm("media-artist")

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	username := auth.GetUsername(c)
	isManager, hasErr := isManager(tx, username, id)
	if hasErr {
		return
	}

	if !isManager {
		errs.PublicError(c, missingPermissionError)
		return
	}

	var media int
	var hasRow bool
	if tx.QueryRow("SELECT i.media FROM playlists p JOIN playlist_items i ON p.current = i.id WHERE p.id = $1", id).Scan(&hasRow, &media) {
		return
	}

	if !hasRow {
		errs.PublicError(c, noCurrentMediaError)
		return
	}

	if tx.Exec(nil, "INSERT INTO alt_metadata (playlist, media, alt_title, alt_artist) VALUES ($1, $2, $3, $4) ON CONFLICT (playlist, media) DO UPDATE SET alt_title = excluded.alt_title, alt_artist = excluded.alt_artist", id, media, title, artist) {
		return
	}

	if tx.Commit() {
		return
	}

	playlistWatchController(c)
	WebSocketPlaylistEvent(id, PlaylistChanged)
	Toast(c, ToastInfo, "Metadata updated", "Metadata of current playlist item was updated successfully")
}

func playlistNextRequest(c *gin.Context) {
	quiet := c.PostForm("quiet") == "true"
	id := stores.GetPlaylistId(c)

	if !auth.IsLoggedIn(c) {
		errs.PublicError(c, unauthorizedError)
		return
	}

	username := auth.GetUsername(c)
	err := NextRequest(id, username)
	if err != nil {
		errs.PublicError(c, err)
		return
	}

	if !quiet {
		Toast(c, ToastInfo, "Next request sent", "Successfully sent next request")
	}
}

func PlaylistUpdateCurrent(playlist int, sign, sortOrder string) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return db.GenericError
	}
	defer tx.Rollback()

	var currentOrder int

	var current sql.NullInt32
	err = tx.QueryRow("SELECT current FROM playlists WHERE id = $1", playlist).Scan(&current)
	if err == sql.ErrNoRows {
		return invalidPlaylistError
	}

	if err != nil {
		return err
	}

	if !current.Valid {
		return noCurrentMediaError
	}

	err = tx.QueryRow("SELECT item_order FROM playlist_items WHERE id = $1", current).Scan(&currentOrder)
	if err != nil {
		return db.GenericError
	}

	var next int
	err = tx.QueryRow("SELECT id FROM playlist_items WHERE playlist = $1 AND item_order "+sign+" $2 ORDER BY item_order "+sortOrder, playlist, currentOrder).Scan(&next)
	if err == sql.ErrNoRows {
		err = tx.QueryRow("SELECT id FROM playlist_items WHERE playlist = $1 ORDER BY item_order "+sortOrder, playlist).Scan(&next)
	}

	if err != nil {
		return db.GenericError
	}

	var payload MediaChangedPayload
	err = tx.QueryRow(`SELECT m.media_type, m.aspect_ratio, m.url FROM playlist_items i JOIN medias m ON i.media = m.id WHERE i.id = $1`, next).Scan(&payload.Type, &payload.AspectRatio, &payload.Url)
	if err != nil {
		return db.GenericError
	}

	_, err = tx.Exec("UPDATE playlists SET current = $1 WHERE id = $2", next, playlist)
	if err != nil {
		return db.GenericError
	}

	if err = tx.Commit(); err != nil {
		return db.GenericError
	}

	WebSocketMediaChange(playlist, "", payload)

	return nil
}

func playlistPrev(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	if !auth.IsLoggedIn(c) {
		errs.PublicError(c, unauthorizedError)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	username := auth.GetUsername(c)
	isManager, hasErr := isManager(tx, username, id)
	if hasErr {
		return
	}

	if !isManager {
		errs.PublicError(c, missingPermissionError)
		return
	}

	if tx.Commit() {
		return
	}

	err := PlaylistUpdateCurrent(id, "<", "DESC")
	if err != nil {
		errs.PublicError(c, err)
		return
	}

	noswap(c)
}

func playlistNext(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	if !auth.IsLoggedIn(c) {
		errs.PublicError(c, unauthorizedError)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	username := auth.GetUsername(c)
	isManager, hasErr := isManager(tx, username, id)
	if hasErr {
		return
	}

	if !isManager {
		errs.PublicError(c, missingPermissionError)
		return
	}

	if tx.Commit() {
		return
	}

	err := PlaylistUpdateCurrent(id, ">", "ASC")
	if err != nil {
		errs.PublicError(c, err)
		return
	}

	noswap(c)
}

func playlistManagers(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var owner string
	var hasRow bool
	if tx.QueryRow("SELECT owner_username FROM playlists WHERE id = $1", id).Scan(&hasRow, &owner) {
		return
	}

	if !hasRow {
		errs.PublicError(c, invalidPlaylistError)
		return
	}

	var rows *sql.Rows
	if tx.Query(&rows, "SELECT username FROM playlist_manage WHERE playlist = $1", id) {
		return
	}

	var managers []string
	for rows.Next() {
		var manager string
		err := rows.Scan(&manager)
		if err != nil {
			db.DatabaseError(c, err)
		}
		managers = append(managers, manager)
	}

	if tx.Commit() {
		return
	}

	html.Render(playlistWatchTmpl, c, "managers", gin.H{
		"Id":       id,
		"Owner":    owner,
		"Managers": managers,
	})
}

func playlistManagerAdd(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	username, err := HxPrompt(c)
	if err != nil {
		errs.PublicError(c, err)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var dummy int
	var hasRow bool
	if tx.QueryRow("SELECT 1 FROM users WHERE username = $1", username).Scan(&hasRow, &dummy) {
		return
	}

	if !hasRow {
		errs.PublicError(c, fmt.Errorf("User '%s' not found", username))
		return
	}

	if tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1", id).Scan(&hasRow, &dummy) {
		return
	}

	if !hasRow {
		errs.PublicError(c, invalidPlaylistError)
		return
	}

	if tx.Exec(nil, "INSERT INTO playlist_manage (playlist, username) VALUES ($1, $2)", id, username) {
		return
	}

	if tx.Commit() {
		return
	}

	noswap(c)
	WebSocketPlaylistEvent(id, ManagersChanged)
	Toast(c, ToastInfo, "New manager successfully added", template.HTML(template.HTMLEscapeString(fmt.Sprintf("User '%s' is now a manager of the playlist", username))))
}

func playlistManagerDelete(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var numAffected int
	for manager, value := range c.Request.URL.Query() {
		if !slices.Contains(value, "on") {
			continue
		}

		var res sql.Result
		if tx.Exec(&res, "DELETE FROM playlist_manage WHERE playlist = $1 AND username = $2", id, manager) {
			return
		}

		affected, err := res.RowsAffected()
		if err != nil {
			db.DatabaseError(c, err)
			return
		}

		numAffected += int(affected)
	}

	if tx.Commit() {
		return
	}

	if numAffected > 0 {
		WebSocketPlaylistEvent(id, ManagersChanged)
	}

	Toast(c, ToastInfo, "Managers successfully removed", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d manager(s) are removed from playlist", numAffected))))
}

func playlistWatchQueueCurrent(c *gin.Context) {
	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	id := stores.GetPlaylistId(c)

	var current sql.NullInt32
	var hasRow bool
	if tx.QueryRow("SELECT current FROM playlists WHERE id = $1", id).Scan(&hasRow, &current) {
		return
	}

	if !hasRow {
		errs.PublicError(c, invalidPlaylistError)
		return
	}

	if !current.Valid {
		errs.PublicError(c, noCurrentMediaError)
		return
	}

	var index int
	if tx.QueryRow("SELECT COALESCE(COUNT(*), 0) FROM playlist_items WHERE item_order < (SELECT item_order FROM playlist_items WHERE id = $1 AND playlist = $2) AND playlist = $2", current, id).Scan(&hasRow, &index) {
		return
	}

	if tx.Commit() {
		return
	}

	limit := 10
	playlistRenderQueue(c, id, index/limit+1)
}

type MoveItem struct {
	id    int
	order int
}

func playlistMoveUp(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	items, err := getCheckedItems(c)
	if err != nil {
		errs.PublicError(c, err)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var moveItems []MoveItem
	for _, itemId := range items {
		var item MoveItem
		item.id = itemId
		if tx.QueryRow("SELECT item_order FROM playlist_items WHERE playlist = $1 AND id = $2", id, itemId).Scan(nil, &item.order) {
			return
		}

		moveItems = append(moveItems, item)
	}

	sort.Slice(moveItems, func(i, j int) bool {
		return moveItems[i].order > moveItems[j].order
	})

	prevAfter := -1

	affected := make(map[int]struct{})

	for _, item := range moveItems {
		var afterId int
		var after int
		var hasRow bool
		if tx.QueryRow("SELECT id, item_order FROM playlist_items WHERE playlist = $1 AND item_order > (SELECT item_order FROM playlist_items WHERE id = $2) ORDER BY item_order ASC LIMIT 1", id, item.id).Scan(&hasRow, &afterId, &after) {
			return
		}
		if !hasRow || afterId == prevAfter {
			prevAfter = item.id
			continue
		}

		if tx.Exec(nil, `
		UPDATE playlist_items SET item_order = (CASE id
			WHEN $1 THEN (SELECT item_order FROM playlist_items WHERE id = $2)
			WHEN $2 THEN (SELECT item_order FROM playlist_items WHERE id = $1)
		END)::INTEGER WHERE id IN ($1, $2)
		`, item.id, afterId) {
			return
		}

		affected[item.id] = struct{}{}
		affected[afterId] = struct{}{}
	}

	if tx.Commit() {
		return
	}

	WebSocketPlaylistEvent(id, PlaylistChanged)
	Toast(c, ToastInfo, "Playlist items reordered", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d item(s) affected", len(affected)))))
}

func playlistMoveDown(c *gin.Context) {
	id := stores.GetPlaylistId(c)

	items, err := getCheckedItems(c)
	if err != nil {
		errs.PublicError(c, err)
		return
	}

	tx := db.BeginTx(c)
	if tx == nil {
		return
	}
	defer tx.Rollback()

	var moveItems []MoveItem
	for _, itemId := range items {
		var item MoveItem
		var hasRow bool
		item.id = itemId
		if tx.QueryRow("SELECT item_order FROM playlist_items WHERE playlist = $1 AND id = $2", id, itemId).Scan(&hasRow, &item.order) {
			return
		}
		moveItems = append(moveItems, item)
	}

	sort.Slice(moveItems, func(i, j int) bool {
		return moveItems[i].order < moveItems[j].order
	})

	prevAfter := -1
	affected := make(map[int]struct{})
	for _, item := range moveItems {
		var afterId int
		var after int
		var hasRow bool
		if tx.QueryRow("SELECT id, item_order FROM playlist_items WHERE playlist = $1 AND item_order < (SELECT item_order FROM playlist_items WHERE id = $2) ORDER BY item_order DESC LIMIT 1", id, item.id).Scan(&hasRow, &afterId, &after) {
			return
		}

		if !hasRow || afterId == prevAfter {
			prevAfter = item.id
			continue
		}

		if tx.Exec(nil, `
		UPDATE playlist_items SET item_order = (CASE id
			WHEN $1 THEN (SELECT item_order FROM playlist_items WHERE id = $2)
			WHEN $2 THEN (SELECT item_order FROM playlist_items WHERE id = $1)
		END)::INTEGER WHERE id IN ($1, $2)
		`, item.id, afterId) {
			return
		}

		affected[item.id] = struct{}{}
		affected[afterId] = struct{}{}
	}

	if tx.Commit() {
		return
	}

	WebSocketPlaylistEvent(id, PlaylistChanged)
	Toast(c, ToastInfo, "Playlist items reordered", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d item(s) affected", len(affected)))))
}
