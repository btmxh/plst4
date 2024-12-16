package routes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/media"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

var ErrHashMismatch = errors.New("Hash mismatch")
var watchTemplate = getTemplate("watch", "templates/watch.tmpl")
var playlistWatchTmpl = getTemplate("watch", "templates/playlists/watch.tmpl")
var playlistWatchInvalidTmpl = getTemplate("watch", "templates/playlists/watch_invalid.tmpl")
var playlistQueryResult = getTemplate("playlist_query_result", "templates/playlists/playlist_query_result.tmpl")

func triggerRefresh(c *gin.Context) {
	c.Header("Hx-Reswap", "none")
	c.Header("Hx-Trigger", "refresh-playlist")
}

func getCheckedItems(c *gin.Context) ([]int, error) {
	var args map[string][]string
	if c.Request.Method == "DELETE" {
		args = c.Request.URL.Query()
	} else {
		args = c.Request.PostForm
	}

	var items []int
	for key, value := range args {
		if value[0] != "on" {
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

func WatchRouter(g *gin.RouterGroup) {
	g.GET("/", SSRRoute(watchTemplate, "layout", gin.H{}))
	g.GET("/:id/", playlistWatch)
	g.GET("/:id/controller", playlistWatchController)
	g.PATCH("/:id/controller/rename", func(c *gin.Context) {
		renamePlaylist(c)
		playlistWatchController(c)
	})
	g.DELETE("/:id/controller/delete", func(c *gin.Context) {
		deletePlaylist(c)
		Redirect(c, "/watch")
	})
	g.GET("/:id/queue", playlistWatchQueue)
	g.POST("/:id/queue/add", playlistAdd)
	g.DELETE("/:id/queue/delete", playlistDelete)
	g.PATCH("/:id/queue/goto/:item-id", playlistGoto)
}

func PlaylistRouter(g *gin.RouterGroup) {
	g.GET("/search", search)
	g.POST("/new", newPlaylist)
	g.PATCH("/:id/rename", func(c *gin.Context) {
		renamePlaylist(c)
		Refresh(c)
	})
	g.DELETE("/:id/delete", func(c *gin.Context) {
		deletePlaylist(c)
		Refresh(c)
	})
}

func search(c *gin.Context) {
	username, _ := middlewares.GetAuthUsername(c)

	query := strings.ToLower(c.Query("query"))
	filter := c.Query("filter")
	offsetStr := c.DefaultQuery("offset", "0")

	limit := 10

	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to fetch playlists", msg)
	}

	offset, err := strconv.Atoi(offsetStr)

	if err != nil {
		fail("Invalid offset value", err)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var rows *sql.Rows
	if filter == string(All) {
		rows, err = tx.Query(
			`SELECT id, name, owner_username, created_timestamp, 
                (SELECT COUNT(*) FROM playlist_items WHERE playlist = playlists.id),
								(SELECT SUM(m.duration) FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = playlists.id)
         FROM playlists 
         WHERE POSITION($1 IN LOWER(name)) > 0 
         ORDER BY created_timestamp 
         LIMIT $2 OFFSET $3`,
			query, limit+1, offset,
		)
	} else if filter == string(Owned) {
		rows, err = tx.Query(
			`SELECT id, name, owner_username, created_timestamp, 
                (SELECT COUNT(*) FROM playlist_items WHERE playlist = playlists.id),
								(SELECT SUM(m.duration) FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = playlists.id)
         FROM playlists 
         WHERE POSITION($1 IN LOWER(name)) > 0 
           AND owner_username = $4 
         ORDER BY created_timestamp 
         LIMIT $2 OFFSET $3`,
			query, limit+1, offset, username,
		)
	} else if filter == string(Managed) {
		rows, err = tx.Query(
			`SELECT id, name, owner_username, created_timestamp, 
                (SELECT COUNT(*) FROM playlist_items WHERE playlist = playlists.id),
								(SELECT SUM(m.duration) FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = playlists.id)
         FROM playlists 
         WHERE POSITION($1 IN LOWER(name)) > 0 
           AND owner_username = $4 
         ORDER BY created_timestamp 
         LIMIT $2 OFFSET $3`,
			query, limit+1, offset, username,
		)
	} else {
		fail("Invalid filter type", nil)
	}

	if err != nil {
		fail("", err)
		return
	}

	var playlists []QueriedPlaylist
	for rows.Next() {
		var playlist QueriedPlaylist
		var totalLength int
		err = rows.Scan(&playlist.Id, &playlist.Name, &playlist.OwnerUsername, &playlist.CreatedTimestamp, &playlist.ItemCount, &totalLength)
		playlist.TotalLength = time.Duration(totalLength) * time.Second
		if err != nil {
			fail("", err)
		}
		playlists = append(playlists, playlist)
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
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

	SSR(playlistQueryResult, c, "content", args)
}

func newPlaylist(c *gin.Context) {
	username, loggedIn := middlewares.GetAuthUsername(c)
	name := c.Request.Header.Get("Hx-Prompt")
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to create new playlist", msg)
	}

	if !loggedIn {
		fail("You must be logged in to create a new playlist.", nil)
		return
	}

	if len(name) == 0 {
		fail("Playlist name could not be empty.", nil)
		return
	}

	if len(name) > 100 {
		fail("Playlist name could not have more than 100 characters", nil)
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var id int
	err = tx.QueryRow("INSERT INTO playlists (name, owner_username) VALUES ($1, $2) RETURNING id", name, username).Scan(&id)
	if err != nil {
		fail("", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
		return
	}

	Redirect(c, "/watch/"+strconv.Itoa(id))
}

func renamePlaylist(c *gin.Context) bool {
	username, loggedIn := middlewares.GetAuthUsername(c)
	name := c.Request.Header.Get("Hx-Prompt")
	id := c.Param("id")
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist rename error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to rename playlist", msg)
	}

	if !loggedIn {
		fail("You must be logged in to do this", nil)
		return false
	}

	if len(name) == 0 {
		fail("Playlist name must not be empty", nil)
		return false
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return false
	}
	defer tx.Rollback()

	row, err := tx.Exec("UPDATE playlists SET name = $1 WHERE id = $2 AND owner_username = $3", name, id, username)
	if err != nil {
		fail("", err)
		return false
	}

	if affected, err := row.RowsAffected(); affected == 0 || err != nil {
		fail("", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
		return false
	}

	return true
}

func deletePlaylist(c *gin.Context) bool {
	username, loggedIn := middlewares.GetAuthUsername(c)
	id := c.Param("id")
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist delete error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		c.Header("Hx-Reswap", "none")
		Toast(c, ToastError, "Unable to rename playlist", msg)
	}

	if !loggedIn {
		fail("You must be logged in to delete this playlist.", nil)
		return false
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return false
	}
	defer tx.Rollback()

	row, err := tx.Exec("DELETE FROM playlists WHERE id = $1 AND owner_username = $2", id, username)
	if err != nil {
		fail("", err)
		return false
	}

	if affected, err := row.RowsAffected(); affected == 0 || err != nil {
		fail("", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
		return false
	}

	return true
}

func playlistWatch(c *gin.Context) {
	id := c.Param("id")
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		SSR(playlistWatchInvalidTmpl, c, "layout", defaultErrorMsg(msg))
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var dummy int
	err = tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1", id).Scan(&dummy)
	if err != nil {
		fail("", err)
		return
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	SSR(playlistWatchTmpl, c, "layout", gin.H{
		"Id": id,
	})
}

func nullUUIDToString(u uuid.NullUUID) string {
	if u.Valid {
		return ""
	}

	return u.UUID.String()
}

func mustJson(args gin.H) string {
	j, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}

	return string(j)
}

func playlistWatchQueue(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist next page fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to fetch playlist queue", msg)
	}

	id := c.Param("id")
	limit := 10
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		fail("Invalid page number", err)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var current sql.NullInt32
	var owner string
	err = tx.QueryRow("SELECT current, owner_username FROM playlists WHERE id = $1", id).Scan(&current, &owner)
	if err == sql.ErrNoRows {
		fail("Playlist not found. Please refresh the page.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	var itemCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM playlist_items WHERE playlist = $1", id).Scan(&itemCount)

	if page == 0 {
		// last page
		page = (itemCount + limit - 1) / limit
		slog.Warn("page", "p", itemCount)
	}

	offset := (page - 1) * 10

	var items []QueuePlaylistItem
	rows, err := tx.Query("SELECT i.id, m.title, m.artist, m.url, m.duration FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = $1 ORDER BY i.item_order OFFSET $2 LIMIT $3", id, offset, limit)
	for rows.Next() {
		var item QueuePlaylistItem
		var duration time.Duration
		rows.Scan(&item.Id, &item.Title, &item.Artist, &item.URL, &duration)
		item.Duration = time.Duration(duration) * time.Second
		item.Index = offset
		offset += 1
		items = append(items, item)
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	currentId := 0
	if current.Valid {
		currentId = int(current.Int32)
	}
	slices.Reverse(items)
	args := gin.H{
		"Id":       id,
		"Items":    items,
		"ThisPage": page,
		"Current":  currentId,
		"Owner":    owner,
	}

	if page > 1 {
		args["PrevPage"] = page - 1
	}
	if itemCount > page*limit {
		args["NextPage"] = page + 1
	}

	SSR(playlistWatchTmpl, c, "queue", args)
}

func playlistWatchController(c *gin.Context) {
	id := c.Param("id")

	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist fetch error", "msg", msg, "err", err)
		SSR(playlistWatchInvalidTmpl, c, "layout", defaultErrorMsg(msg))
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var name string
	var owner string
	var createdTimestamp time.Time
	var current sql.NullInt32
	err = tx.QueryRow("SELECT name, owner_username, created_timestamp, current FROM playlists WHERE id = $1", id).Scan(&name, &owner, &createdTimestamp, &current)
	if err != nil {
		fail("", err)
		return
	}

	args := gin.H{
		"Id":               id,
		"Name":             name,
		"Owner":            owner,
		"CreatedTimestamp": createdTimestamp,
	}

	if current.Valid {
		var mediaId int
		var title string
		var artist string
		var duration int
		var url string
		err = tx.QueryRow("SELECT m.id, m.title, m.artist, m.duration, m.url FROM playlist_items i JOIN medias m ON i.media = m.id WHERE i.id = $1", current).Scan(&mediaId, &title, &artist, &duration, &url)
		if err != nil {
			fail("", err)
			return
		}
		args["Media"] = gin.H{
			"Id":             mediaId,
			"ItemId":         current.Int32,
			"Type":           "yt",
			"URL":            url,
			"Title":          title,
			"Artist":         artist,
			"OriginalTitle":  title,
			"OriginalArtist": artist,
			"Duration":       time.Duration(duration) * time.Second,
		}
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	SSR(playlistWatchTmpl, c, "controller", args)
}

func localRebalance(tx *sql.Tx, playlist string, startOrder int, endOrder int, numInsert int) error {
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

	slog.Warn("unique", "s", startOrder, "e", endOrder, "D", density)
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

func playlistAddBackground(playlist string, socketId string, canonInfo *media.MediaCanonicalizeInfo, pos PlaylistAddPosition) {
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
				err = tx.QueryRow("INSERT INTO medias (title, artist, duration, url) VALUES ($1, $2, $3, $4) RETURNING id", entry.ResolveInfo.Title, entry.ResolveInfo.Artist, int(entry.ResolveInfo.Duration.Seconds()), entry.CanonInfo.Url).Scan(&id)
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

			if nextOrder == currentOrder+1 {
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

	err = WebSocketEvent(socketId, "refresh-playlist")
	if err != nil {
		slog.Warn("Unable to send refresh-playlist event to client", "sid", socketId)
	}
	WebSocketToast(socketId, ToastInfo, "Media added successfully", template.HTML(template.HTMLEscapeString(msg)))
}

func playlistAdd(c *gin.Context) {
	id := c.Param("id")
	username, loggedIn := middlewares.GetAuthUsername(c)
	pos := c.PostForm("add-position")
	url := c.PostForm("url")
	wsId := c.PostForm("websocket-id")

	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist item add error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		triggerRefresh(c)
		Toast(c, ToastError, "Unable to add media to playlist", msg)
	}

	if !loggedIn {
		fail("You must be logged in to add media to this playlist.", nil)
		return
	}

	if pos != string(AddToStart) && pos != string(AddToEnd) && pos != string(QueueNext) {
		fail("Invalid add position", nil)
		return
	}

	if wsId == "" {
		fail("Missing WebSocket identifier. Please check your connection", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var dummy int
	err = tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1 AND owner_username = $2", id, username).Scan(&dummy)
	if err == sql.ErrNoRows {
		fail("You must be a manager of this playlist to modify its content.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	canonInfo, err := media.CanonicalizeMedia(url)
	if err != nil {
		fail("Unable to canonicalize the media URL", err)
		return
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	go playlistAddBackground(id, wsId, canonInfo, PlaylistAddPosition(pos))
	Toast(c, ToastInfo, "Media added successfully", template.HTML(template.HTMLEscapeString(fmt.Sprintf("Adding media with URL %s to playlist...", url))))
}

func playlistGoto(c *gin.Context) {
	id := c.Param("id")

	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist goto error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		triggerRefresh(c)
		Toast(c, ToastError, "Unable to change current item in playlist", msg)
	}

	username, loggedIn := middlewares.GetAuthUsername(c)
	if !loggedIn {
		fail("You must be logged in to set current media of this playlist.", nil)
		return
	}

	itemId, err := strconv.Atoi(c.Param("item-id"))
	if err != nil {
		fail("Invalid item ID", err)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var dummy int
	err = tx.QueryRow("SELECT 1 FROM playlist_items WHERE id = $1 AND playlist = $2", itemId, id).Scan(&dummy)
	if err != nil {
		fail("", err)
		return
	}

	_, err = tx.Exec("UPDATE playlists SET current = $1 WHERE id = $2 AND owner_username = $3", itemId, id, username)
	if err != nil {
		fail("", err)
		return
	}

	var title string
	var artist string
	err = tx.QueryRow("SELECT m.title, m.artist FROM playlist_items i JOIN medias m ON i.media = m.id WHERE i.id = $1 AND i.playlist = $2", itemId, id).Scan(&title, &artist)
	if err != nil {
		fail("Item does not belong to playlist", err)
		return
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	triggerRefresh(c)
	Toast(c, ToastInfo, "Playlist current media changed", template.HTML(template.HTMLEscapeString(fmt.Sprintf("Current media is now %s - %s", title, artist))))
}

func playlistDelete(c *gin.Context) {
	id := c.Param("id")

	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist delete error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		triggerRefresh(c)
		Toast(c, ToastError, "Unable to delete items from playlist", msg)
	}

	username, loggedIn := middlewares.GetAuthUsername(c)
	if !loggedIn {
		fail("You must be logged in to modify this playlist.", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var dummy int
	err = tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1 AND owner_username = $2", id, username).Scan(&dummy)
	if err == sql.ErrNoRows {
		fail("You must be a manager of this playlist to modify its content.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	items, err := getCheckedItems(c)
	for _, item := range items {
		_, err = tx.Exec("DELETE FROM playlist_items WHERE id = $1 AND playlist = $2", item, id)
		if err != nil {
			fail("", err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	triggerRefresh(c)
	Toast(c, ToastInfo, "Playlist items removed", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d item(s) are removed from the playlist", len(items)))))
}
