package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/media"
	"github.com/btmxh/plst4/internal/middlewares"
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

var ErrHashMismatch = errors.New("Hash mismatch")
var watchTemplate = getTemplate("watch", "templates/watch.tmpl")
var playlistWatchTmpl = getTemplate("watch", "templates/playlists/watch.tmpl")
var playlistWatchInvalidTmpl = getTemplate("watch", "templates/playlists/watch_invalid.tmpl")
var playlistQueryResult = getTemplate("playlist_query_result", "templates/playlists/playlist_query_result.tmpl")

func noswap(c *gin.Context) {
	c.Header("Hx-Reswap", "none")
}

func isManager(tx *sql.Tx, username string, playlist int) (bool, error) {
	var dummy int
	err := tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1 AND owner_username = $2", playlist, username).Scan(&dummy)
	if err == nil {
		return true, nil
	}

	if err != sql.ErrNoRows {
		return false, err
	}

	err = tx.QueryRow("SELECT 1 FROM playlist_manage WHERE playlist = $1 AND username = $2", playlist, username).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}

	if err == nil {
		return true, nil
	}

	return false, err
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
	g.GET("/", SSRRoute(watchTemplate, "layout", gin.H{}))
	g.GET("/:id/", playlistWatch)
	g.GET("/:id/controller", playlistWatchController)
	g.POST("/:id/controller/submit", playlistSubmitMetadata)
	g.PATCH("/:id/controller/rename", func(c *gin.Context) {
		name, renamed := renamePlaylist(c)
		if renamed {
			updateTitle(c, fmt.Sprintf("plst4 - %s", name))
		}
		playlistWatchController(c)
	})
	g.DELETE("/:id/controller/delete", func(c *gin.Context) {
		deletePlaylist(c)
		Redirect(c, "/watch")
	})
	g.GET("/:id/queue", playlistWatchQueue)
	g.GET("/:id/queue/current", playlistWatchQueueCurrent)
	g.POST("/:id/queue/add", playlistAdd)
	g.DELETE("/:id/queue/delete", playlistDelete)
	g.PATCH("/:id/queue/goto/:item-id", playlistGoto)
	g.POST("/:id/queue/nextreq", playlistNextRequest)
	g.POST("/:id/queue/prev", playlistPrev)
	g.POST("/:id/queue/next", playlistNext)
	g.POST("/:id/queue/up", playlistMoveUp)
	g.POST("/:id/queue/down", playlistMoveDown)
	g.GET("/:id/managers", playlistManagers)
	g.POST("/:id/managers/add", playlistManagerAdd)
	g.DELETE("/:id/managers/delete", playlistManagerDelete)
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
              COALESCE((SELECT COUNT(*) FROM playlist_items WHERE playlist = playlists.id), 0),
							COALESCE((SELECT SUM(m.duration) FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = playlists.id), 0)
         FROM playlists 
         WHERE POSITION($1 IN LOWER(name)) > 0 
         ORDER BY created_timestamp 
         LIMIT $2 OFFSET $3`,
			query, limit+1, offset,
		)
	} else if filter == string(Owned) {
		rows, err = tx.Query(
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
		rows, err = tx.Query(
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
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to create new playlist", msg)
	}

	name, err := getHxPrompt(c)
	if err != nil {
		fail("Invalid playlist name", err)
		return
	}

	username, loggedIn := middlewares.GetAuthUsername(c)
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

func getHxPrompt(c *gin.Context) (string, error) {
	return url.PathUnescape(c.GetHeader("Hx-Prompt"))
}

func renamePlaylist(c *gin.Context) (string, bool) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist rename error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to rename playlist", msg)
	}

	username, loggedIn := middlewares.GetAuthUsername(c)

	if !loggedIn {
		fail("You must be logged in to rename this playlist", nil)
		return "", false
	}

	name, err := getHxPrompt(c)
	if err != nil {
		fail("Invalid playlist name", err)
		return "", false
	}

	if len(name) == 0 {
		fail("Playlist name must not be empty", nil)
		return "", false
	}

	id := c.Param("id")

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return "", false
	}
	defer tx.Rollback()

	row, err := tx.Exec("UPDATE playlists SET name = $1 WHERE id = $2 AND owner_username = $3", name, id, username)
	if err != nil {
		fail("", err)
		return "", false
	}

	if affected, err := row.RowsAffected(); affected == 0 || err != nil {
		fail("", err)
		return "", false
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
		return "", false
	}

	return name, true
}

func deletePlaylist(c *gin.Context) bool {
	username, loggedIn := middlewares.GetAuthUsername(c)
	id := c.Param("id")
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist delete error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to delete playlist", msg)
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

	var name string
	err = tx.QueryRow("SELECT name FROM playlists WHERE id = $1", id).Scan(&name)
	if err != nil {
		fail("", err)
		return
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	SSR(playlistWatchTmpl, c, "layout", gin.H{
		"Id":    id,
		"Title": name,
	})
}

func playlistSSRQueue(c *gin.Context, playlist int, page int) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist next page fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		noswap(c)
		Toast(c, ToastError, "Unable to fetch playlist queue", msg)
	}

	limit := 10
	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	username, _ := middlewares.GetAuthUsername(c)
	isManager, err := isManager(tx, username, playlist)
	if err != nil {
		fail("", err)
		return
	}

	var current sql.NullInt32
	var owner string
	err = tx.QueryRow("SELECT current, owner_username FROM playlists WHERE id = $1", playlist).Scan(&current, &owner)
	if err == sql.ErrNoRows {
		fail("Playlist not found. Please refresh the page.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	var itemCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM playlist_items WHERE playlist = $1", playlist).Scan(&itemCount)

	if page == 0 {
		// last page
		page = (itemCount + limit - 1) / limit
		slog.Warn("page", "p", itemCount)
	}

	offset := (page - 1) * 10

	var items []QueuePlaylistItem
	rows, err := tx.Query(`
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
    OFFSET $2 LIMIT $3`, playlist, offset, limit)

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

	SSR(playlistWatchTmpl, c, "queue", args)
}

func playlistWatchQueue(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist next page fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		noswap(c)
		Toast(c, ToastError, "Unable to fetch playlist queue", msg)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("Invalid playlist ID", err)
		return
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		fail("Invalid page number", err)
		return
	}

	playlistSSRQueue(c, id, page)
}

func playlistWatchController(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to fetch playlist controller", msg)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("Invalid playlist ID", err)
		return
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

	username, _ := middlewares.GetAuthUsername(c)
	isManager, err := isManager(tx, username, id)
	if err != nil {
		fail("", err)
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
		err = tx.QueryRow(`
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
			WHERE i.id = $1`, current).Scan(&mediaId, &title, &artist, &altTitle, &altArtist, &duration, &url, &mediaAddTimestamp, &itemAddTimestamp)
		if err != nil {
			fail("", err)
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

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	SSR(playlistWatchTmpl, c, "controller", args)
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

	WebSocketPlaylistEvent(playlist, PlaylistChanged)
	WebSocketToast(socketId, ToastInfo, "Media added successfully", template.HTML(template.HTMLEscapeString(msg)))
}

func playlistAdd(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist item add error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to add media to playlist", msg)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("Invalid playlist ID", err)
		return
	}

	username, loggedIn := middlewares.GetAuthUsername(c)
	pos := c.PostForm("add-position")
	url := c.PostForm("url")
	wsId := c.PostForm("websocket-id")

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
	err = tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1", id).Scan(&dummy)
	if err == sql.ErrNoRows {
		fail("Playlist not found.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	isManager, err := isManager(tx, username, id)
	if err != nil {
		fail("", err)
		return
	}

	if !isManager {
		fail("You must be a manager of this playlist to add media", nil)
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
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist goto error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to change current item in playlist", msg)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("Invalid playlist ID", err)
		return
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

	isManager, err := isManager(tx, username, id)
	if err != nil {
		fail("", err)
		return
	}

	if !isManager {
		fail("You must be the manager of this playlist to set the current item", nil)
		return
	}

	var dummy int
	err = tx.QueryRow("SELECT 1 FROM playlist_items WHERE id = $1 AND playlist = $2", itemId, id).Scan(&dummy)
	if err != nil {
		fail("", err)
		return
	}

	_, err = tx.Exec("UPDATE playlists SET current = $1 WHERE id = $2", itemId, id)
	if err != nil {
		fail("", err)
		return
	}

	var title string
	var artist string
	err = tx.QueryRow(`
		SELECT
			COALESCE(a.alt_title, m.title),
			COALESCE(a.alt_artist, m.artist)
		FROM playlist_items i
		JOIN medias m ON i.media = m.id
		LEFT JOIN alt_metadata a ON a.media = m.id AND a.playlist = i.playlist
		WHERE i.id = $1 and i.playlist = $2
	`, itemId, id).Scan(&title, &artist)
	if err != nil {
		fail("Item does not belong to playlist", err)
		return
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	go sendMediaChanged(id, itemId)
}

func playlistDelete(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist delete error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to delete items from playlist", msg)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("Invalid playlist ID", err)
		return
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
	err = tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1", id).Scan(&dummy)
	if err == sql.ErrNoRows {
		fail("Playlist does not exist.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	isManager, err := isManager(tx, username, id)
	if err != nil {
		fail("", err)
		return
	}

	if !isManager {
		fail("You must be a manager to remove media from playlists", nil)
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

	WebSocketPlaylistEvent(id, PlaylistChanged)
	Toast(c, ToastInfo, "Playlist items removed", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d item(s) are removed from the playlist", len(items)))))
}

func playlistSubmitMetadata(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist metadata submit error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to submit metadata to playlist", msg)
	}

	username, loggedIn := middlewares.GetAuthUsername(c)
	if !loggedIn {
		fail("You must be logged in to change media metadata", nil)
		return
	}

	title := c.PostForm("media-title")
	if len(title) == 0 {
		fail("New title must not be empty", nil)
		return
	}

	artist := c.PostForm("media-artist")
	if len(artist) == 0 {
		fail("New artist must not be empty", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("", err)
		return
	}

	isManager, err := isManager(tx, username, id)
	if err != nil {
		fail("", err)
		return
	}

	if !isManager {
		fail("You must be a manager to submit playlist metadata", err)
		return
	}

	var media int
	err = tx.QueryRow("SELECT i.media FROM playlists p JOIN playlist_items i ON p.current = i.id WHERE p.id = $1", id).Scan(&media)
	if err != nil {
		if err == sql.ErrNoRows {
			fail("No current playing media", err)
			return
		}

		fail("", err)
		return
	}

	_, err = tx.Exec("INSERT INTO alt_metadata (playlist, media, alt_title, alt_artist) VALUES ($1, $2, $3, $4) ON CONFLICT (playlist, media) DO UPDATE SET alt_title = excluded.alt_title, alt_artist = excluded.alt_artist", id, media, title, artist)
	if err != nil {
		fail("", err)
		return
	}

	if err = tx.Commit(); err != nil {
		fail("", err)
		return
	}

	playlistWatchController(c)
	WebSocketPlaylistEvent(id, PlaylistChanged)
	Toast(c, ToastInfo, "Metadata updated", "Metadata of current playlist item was updated successfully")
}

func playlistNextRequest(c *gin.Context) {
	fail := func(msg template.HTML, err error, quiet bool) {
		slog.Warn("Playlist next request error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		if !quiet {
			Toast(c, ToastError, "Unable to send next request to server", msg)
		}
	}

	username, loggedIn := middlewares.GetAuthUsername(c)
	quiet := c.PostForm("quiet") == "true"

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("", err, false)
		return
	}

	if !loggedIn {
		fail("You must be logged in to request next media", err, quiet)
		return
	}

	msg, err := NextRequest(id, username)
	if err != nil {
		fail(msg, err, false)
		return
	}

	if !quiet {
		Toast(c, ToastInfo, "Next request sent", "Successfully sent next request")
	}
}

func PlaylistUpdateCurrent(playlist int, sign, sortOrder string) (template.HTML, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var currentOrder int

	var current sql.NullInt32
	err = tx.QueryRow("SELECT current FROM playlists WHERE id = $1", playlist).Scan(&current)
	if err == sql.ErrNoRows {
		return "Invalid playlist ID", err
	}

	if err != nil {
		return "", err
	}

	if !current.Valid {
		return "No current playing media", errors.New("No current playing media")
	}

	err = tx.QueryRow("SELECT item_order FROM playlist_items WHERE id = $1", current).Scan(&currentOrder)
	if err != nil {
		return "", err
	}

	var next int
	err = tx.QueryRow("SELECT id FROM playlist_items WHERE playlist = $1 AND item_order "+sign+" $2 ORDER BY item_order "+sortOrder, playlist, currentOrder).Scan(&next)
	if err == sql.ErrNoRows {
		err = tx.QueryRow("SELECT id FROM playlist_items WHERE playlist = $1 ORDER BY item_order "+sortOrder, playlist).Scan(&next)
	}

	if err != nil {
		return "", err
	}

	var payload MediaChangedPayload
	err = tx.QueryRow(`SELECT m.media_type, m.aspect_ratio, m.url FROM playlist_items i JOIN medias m ON i.media = m.id WHERE i.id = $1`, next).Scan(&payload.Type, &payload.AspectRatio, &payload.Url)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec("UPDATE playlists SET current = $1 WHERE id = $2", next, playlist)
	if err != nil {
		return "", err
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}

	WebSocketMediaChange(playlist, "", payload)

	return "", nil
}

func playlistPrev(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Error skipping prev media", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to return to previous media", msg)
	}

	playlist, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("Invalid playlist ID", err)
		return
	}

	username, loggedIn := middlewares.GetAuthUsername(c)

	if !loggedIn {
		fail("You must be logged in to skip media in playlist", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	isManager, err := isManager(tx, username, playlist)
	if err != nil {
		fail("", err)
		return
	}

	if !isManager {
		fail("You must be a playlist manager to skip media in playlist", nil)
		return
	}

	if err = tx.Commit(); err != nil {
		fail("", err)
		return
	}

	if msg, err := PlaylistUpdateCurrent(playlist, "<", "DESC"); err != nil {
		fail(msg, err)
		return
	}

	noswap(c)
}

func playlistNext(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Error skipping next media", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to skip to next media", msg)
	}

	playlist, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("Invalid playlist ID", err)
		return
	}

	username, loggedIn := middlewares.GetAuthUsername(c)

	if !loggedIn {
		fail("You must be logged in to skip media in playlist", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	isManager, err := isManager(tx, username, playlist)
	if err != nil {
		fail("", err)
		return
	}

	if !isManager {
		fail("You must be a playlist manager to skip media in playlist", nil)
		return
	}

	if err = tx.Commit(); err != nil {
		fail("", err)
		return
	}

	if msg, err := PlaylistUpdateCurrent(playlist, ">", "ASC"); err != nil {
		fail(msg, err)
		return
	}

	noswap(c)
}

func playlistManagers(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Error rendering playlist managers", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		noswap(c)
		Toast(c, ToastError, "Unable to retrieve playlist managers page", msg)
	}

	id := c.Param("id")

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var owner string
	err = tx.QueryRow("SELECT owner_username FROM playlists WHERE id = $1", id).Scan(&owner)
	if err != nil {
		fail("", err)
		return
	}

	rows, err := tx.Query("SELECT username FROM playlist_manage WHERE playlist = $1", id)
	if err != nil {
		fail("", err)
		return
	}

	var managers []string
	for rows.Next() {
		var manager string
		rows.Scan(&manager)
		managers = append(managers, manager)
	}

	if err = tx.Commit(); err != nil {
		fail("", err)
		return
	}

	SSR(playlistWatchTmpl, c, "managers", gin.H{
		"Id":       id,
		"Owner":    owner,
		"Managers": managers,
	})
}

func playlistManagerAdd(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Error adding playlist managers", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		noswap(c)
		Toast(c, ToastError, "Unable to add new playlist manager", msg)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("Invalid playlist ID", nil)
		return
	}

	username, err := getHxPrompt(c)
	if err != nil {
		fail("Invalid username", err)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var dummy int
	err = tx.QueryRow("SELECT 1 FROM users WHERE username = $1", username).Scan(&dummy)
	if err == sql.ErrNoRows {
		fail(template.HTML(template.HTMLEscapeString(fmt.Sprintf("User '%s' not found", username))), err)
		return
	}

	if err != nil {
		fail("", err)
		return
	}

	err = tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1", id).Scan(&dummy)
	if err == sql.ErrNoRows {
		fail("Playlist not found", err)
		return
	}

	_, err = tx.Exec("INSERT INTO playlist_manage (playlist, username) VALUES ($1, $2)", id, username)
	if err != nil {
		fail("", err)
		return
	}

	if err = tx.Commit(); err != nil {
		fail("", err)
		return
	}

	noswap(c)
	WebSocketPlaylistEvent(id, ManagersChanged)
	Toast(c, ToastInfo, "New manager successfully added", template.HTML(template.HTMLEscapeString(fmt.Sprintf("User '%s' is now a manager of the playlist", username))))
}

func playlistManagerDelete(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Error removing playlist managers", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		Toast(c, ToastError, "Unable to remove playlist managers", msg)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("Invalid playlist ID", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var numAffected int
	for manager, value := range c.Request.URL.Query() {
		if !slices.Contains(value, "on") {
			continue
		}

		rows, err := tx.Exec("DELETE FROM playlist_manage WHERE playlist = $1 AND username = $2", id, manager)
		if err != nil {
			fail("", err)
			return
		}

		affected, err := rows.RowsAffected()
		if err != nil {
			fail("", err)
			return
		}

		numAffected += int(affected)
	}

	if err = tx.Commit(); err != nil {
		fail("", err)
		return
	}

	if numAffected > 0 {
		WebSocketPlaylistEvent(id, ManagersChanged)
	}
	Toast(c, ToastInfo, "Managers successfully removed", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d manager(s) are removed from playlist", numAffected))))
}

func playlistWatchQueueCurrent(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Error going to current page", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		noswap(c)
		Toast(c, ToastError, "Unable to go to current page", msg)
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("Invalid playlist ID", err)
		return
	}

	var current sql.NullInt32
	err = tx.QueryRow("SELECT current FROM playlists WHERE id = $1", id).Scan(&current)
	if err != nil {
		fail("", err)
		return
	}

	if !current.Valid {
		fail("No current media is playing", nil)
		return
	}

	var index int
	err = tx.QueryRow("SELECT COALESCE(COUNT(*), 0) FROM playlist_items WHERE item_order < (SELECT item_order FROM playlist_items WHERE id = $1 AND playlist = $2) AND playlist = $2", current, id).Scan(&index)
	if err != nil {
		fail("", err)
		return
	}

	if err = tx.Commit(); err != nil {
		fail("", err)
		return
	}

	limit := 10
	playlistSSRQueue(c, id, index/limit+1)
}

type MoveItem struct {
	id    int
	order int
}

func playlistMoveUp(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Error moving playlist items up", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		noswap(c)
		Toast(c, ToastError, "Unable to move playlist items", msg)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("", err)
		return
	}

	items, err := getCheckedItems(c)
	if err != nil {
		fail("", err)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var moveItems []MoveItem
	for _, itemId := range items {
		var item MoveItem
		item.id = itemId
		err = tx.QueryRow("SELECT item_order FROM playlist_items WHERE playlist = $1 AND id = $2", id, itemId).Scan(&item.order)
		if err != nil {
			fail("", err)
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
		err = tx.QueryRow("SELECT id, item_order FROM playlist_items WHERE playlist = $1 AND item_order > (SELECT item_order FROM playlist_items WHERE id = $2) ORDER BY item_order ASC LIMIT 1", id, item.id).Scan(&afterId, &after)
		if err == sql.ErrNoRows {
			prevAfter = item.id
			continue
		}

		if err != nil {
			fail("", err)
			return
		}

		if afterId == prevAfter {
			prevAfter = item.id
			continue
		}

		_, err := tx.Exec(`
		UPDATE playlist_items SET item_order = (CASE id
			WHEN $1 THEN (SELECT item_order FROM playlist_items WHERE id = $2)
			WHEN $2 THEN (SELECT item_order FROM playlist_items WHERE id = $1)
		END)::INTEGER WHERE id IN ($1, $2)
		`, item.id, afterId)

		affected[item.id] = struct{}{}
		affected[afterId] = struct{}{}

		if err != nil {
			fail("", err)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		fail("", err)
		return
	}

	WebSocketPlaylistEvent(id, PlaylistChanged)
	Toast(c, ToastInfo, "Playlist items reordered", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d item(s) affected", len(affected)))))
}

func playlistMoveDown(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Error moving playlist items down", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		noswap(c)
		Toast(c, ToastError, "Unable to move playlist items", msg)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail("", err)
		return
	}

	items, err := getCheckedItems(c)
	if err != nil {
		fail("", err)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var moveItems []MoveItem
	for _, itemId := range items {
		var item MoveItem
		item.id = itemId
		err = tx.QueryRow("SELECT item_order FROM playlist_items WHERE playlist = $1 AND id = $2", id, itemId).Scan(&item.order)
		if err != nil {
			fail("", err)
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
		err = tx.QueryRow("SELECT id, item_order FROM playlist_items WHERE playlist = $1 AND item_order < (SELECT item_order FROM playlist_items WHERE id = $2) ORDER BY item_order DESC LIMIT 1", id, item.id).Scan(&afterId, &after)
		if err == sql.ErrNoRows {
			prevAfter = item.id
			continue
		}

		if err != nil {
			fail("", err)
			return
		}

		if afterId == prevAfter {
			prevAfter = item.id
			continue
		}

		_, err := tx.Exec(`
		UPDATE playlist_items SET item_order = (CASE id
			WHEN $1 THEN (SELECT item_order FROM playlist_items WHERE id = $2)
			WHEN $2 THEN (SELECT item_order FROM playlist_items WHERE id = $1)
		END)::INTEGER WHERE id IN ($1, $2)
		`, item.id, afterId)

		affected[item.id] = struct{}{}
		affected[afterId] = struct{}{}

		if err != nil {
			fail("", err)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		fail("", err)
		return
	}

	WebSocketPlaylistEvent(id, PlaylistChanged)
	Toast(c, ToastInfo, "Playlist items reordered", template.HTML(template.HTMLEscapeString(fmt.Sprintf("%d item(s) affected", len(affected)))))
}
