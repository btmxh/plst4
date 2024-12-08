package routes

import (
	"crypto"
	"database/sql"
	"encoding/base64"
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
	"github.com/btmxh/plst4/internal/ds"
	"github.com/btmxh/plst4/internal/middlewares"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PlaylistFilter string
type QueriedPlaylist struct {
	Id               uuid.UUID
	Name             string
	OwnerUsername    string
	CreatedTimestamp time.Time
	ItemCount        int
	TotalLength      time.Duration
	CurrentPlaying   string
}

type QueuePlaylistItem struct {
	Title          string
	Artist         string
	URL            string
	Id             uuid.UUID
	Prev           uuid.NullUUID
	Next           uuid.NullUUID
	Index          int
	PositionalHash string
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

func hashSecret(secret string, args ...string) string {
	hasher := crypto.SHA256.New()
	for _, arg := range args {
		fmt.Fprintf(hasher, "%s\000", arg)
	}
	return base64.StdEncoding.EncodeToString(hasher.Sum([]byte(secret)))
}

func positionalHash(secret string, id ds.NonNilId, index int) string {
	return hashSecret(secret, id.String(), strconv.Itoa(index))
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
	g.GET("/:id/queue/firstpage", playlistWatchQueueFirstPage)
	g.GET("/:id/queue/lastpage", playlistWatchQueueLastPage)
	g.GET("/:id/queue/prevpage", playlistWatchQueuePrevPage)
	g.GET("/:id/queue/nextpage", playlistWatchQueueNextPage)
	g.GET("/:id/queue/thispage", playlistWatchQueueNextPage) // the logic is the same
	g.POST("/:id/queue/add", playlistAdd)
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
		rows, err = tx.Query("SELECT id, name, owner_username, created_timestamp, item_count FROM playlists WHERE POSITION($1 IN LOWER(name)) > 0 ORDER BY created_timestamp LIMIT $2 OFFSET $3", query, limit+1, offset)
	} else if filter == string(Owned) {
		rows, err = tx.Query("SELECT id, name, owner_username, created_timestamp, item_count FROM playlists WHERE POSITION($1 IN LOWER(name)) > 0 AND owner_username = $4 ORDER BY created_timestamp LIMIT $2 OFFSET $3", query, limit+1, offset, username)
	} else if filter == string(Managed) {
		rows, err = tx.Query("SELECT id, name, owner_username, created_timestamp, item_count FROM playlists WHERE POSITION($1 IN LOWER(name)) > 0 AND owner_username = $4 ORDER BY created_timestamp LIMIT $2 OFFSET $3", query, limit+1, offset, username)
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
		err = rows.Scan(&playlist.Id, &playlist.Name, &playlist.OwnerUsername, &playlist.CreatedTimestamp, &playlist.ItemCount)
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

	var id uuid.UUID
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

	Redirect(c, "/watch/"+id.String())
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

func parseNullUUID(str string) (uuid.NullUUID, error) {
	if str == "" || str == "null" {
		return uuid.NullUUID{Valid: false}, nil
	}

	id, err := uuid.Parse(str)
	return uuid.NullUUID{Valid: err == nil, UUID: id}, err
}

func nullUUIDToString(u uuid.NullUUID) string {
	if u.Valid {
		return ""
	}

	return u.UUID.String()
}

func queueLimit(_ *gin.Context) int {
	return 10
}

func getPivot(c *gin.Context, secret string) (uuid.UUID, int, error) {
	pivot, err := uuid.Parse(c.Query("pivot"))
	if err != nil {
		return uuid.UUID{}, 0, err
	}

	pivotIndex, err := strconv.Atoi(c.Query("pivot-index"))
	if err != nil {
		return uuid.UUID{}, 0, err
	}

	if c.Query("state-hash") != hashSecret(secret) || c.Query("pivot-pos-hash") != positionalHash(secret, pivot, pivotIndex) {
		return uuid.UUID{}, 0, errors.New("Hash mismatch")
	} else {
		return pivot, pivotIndex, nil
	}
}

func queryItem(tx *sql.Tx, id ds.NonNilId, index int, secret string) (QueuePlaylistItem, error) {
	item := QueuePlaylistItem{Id: id, Index: index}
	err := tx.QueryRow("SELECT i.prev, i.next, m.title, m.artist FROM playlist_items i INNER JOIN medias m ON i.media = m.id WHERE i.id = $1", id).Scan(&item.Prev, &item.Next, &item.Title, &item.Artist)
	item.PositionalHash = positionalHash(secret, id, index)
	return item, err
}

func mustJson(args gin.H) string {
	j, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}

	return string(j)
}

func firstPagePivotArgs() string {
	return mustJson(gin.H{"pivot": "firstpage"})
}

func pivotArgs(pivot ds.NonNilId, index int, secret string) string {
	return mustJson(gin.H{
		"pivot":          pivot.String(),
		"pivot-index":    index,
		"pivot-pos-hash": positionalHash(secret, pivot, index),
	})
}

func playlistWatchQueueFirstPage(c *gin.Context) {
	id := c.Param("id")
	limit := queueLimit(c)

	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist last page fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		c.Header("Hx-Reswap", "none")
		// this is an infinite loop
		// playlistWatchQueueFirstPage(c)
		Toast(c, ToastError, "Unable to fetch playlist queue", msg)
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var items []QueuePlaylistItem
	var nextId ds.Id
	var stateSecret string
	var currentIndex int
	err = tx.QueryRow("SELECT first, state_secret, current_idx FROM playlists WHERE id = $1", id).Scan(&nextId, &stateSecret, &currentIndex)
	if err == sql.ErrNoRows {
		fail("Playlist not found. Please refresh the page.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	i := 0
	for ; nextId.Valid && i < limit; i++ {
		item, err := queryItem(tx, nextId.UUID, i, stateSecret)
		if err != nil {
			fail("", err)
			return
		}

		items = append(items, item)
		nextId = item.Next
	}

	slices.Reverse(items)

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	args := gin.H{
		"Id":         id,
		"GlobalHash": hashSecret(stateSecret),
		"Items":      items,
		"ThisPage":   firstPagePivotArgs(),
		"CurrentIdx": currentIndex,
	}
	if nextId.Valid {
		args["NextPage"] = pivotArgs(nextId.UUID, i, stateSecret)
	}
	SSR(playlistWatchTmpl, c, "queue", args)
}

func playlistWatchQueueLastPage(c *gin.Context) {
	id := c.Param("id")
	limit := queueLimit(c)

	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist first page fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		playlistWatchQueueFirstPage(c)
		Toast(c, ToastError, "Unable to fetch playlist queue", msg)
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var items []QueuePlaylistItem
	var prevId ds.Id
	var stateSecret string
	var itemCount int
	var currentIndex int
	err = tx.QueryRow("SELECT last, state_secret, item_count, current_idx FROM playlists WHERE id = $1", id).Scan(&prevId, &stateSecret, &itemCount, &currentIndex)
	if err == sql.ErrNoRows {
		fail("Playlist not found. Please refresh the page.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	limit = itemCount % limit

	i := 0
	for ; prevId.Valid && i < limit; i++ {
		item, err := queryItem(tx, prevId.UUID, itemCount-1-i, stateSecret)
		if err != nil {
			fail("", err)
			return
		}

		items = append(items, item)
		prevId = item.Prev
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	args := gin.H{
		"Id":         id,
		"GlobalHash": hashSecret(stateSecret),
		"Items":      items,
		"ThisPage":   firstPagePivotArgs(), // if len(items) == 0
		"CurrentIdx": currentIndex,
	}
	if len(items) > 0 {
		args["ThisPage"] = pivotArgs(items[len(items)-1].Id, items[len(items)-1].Index, stateSecret)
	}
	if prevId.Valid {
		args["PrevPage"] = pivotArgs(prevId.UUID, itemCount-i-1, stateSecret)
	}
	SSR(playlistWatchTmpl, c, "queue", args)
}

func playlistWatchQueueNextPage(c *gin.Context) {
	if c.Query("pivot") == "firstpage" {
		playlistWatchQueueFirstPage(c)
		return
	}

	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist next page fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		playlistWatchQueueFirstPage(c)
		Toast(c, ToastError, "Unable to fetch playlist queue", msg)
	}

	id := c.Param("id")
	limit := queueLimit(c)

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var items []QueuePlaylistItem
	var stateSecret string
	var currentIndex int
	// TODO: implement state update tracking
	err = tx.QueryRow("SELECT p.state_secret, p.current_idx FROM playlist_items i INNER JOIN playlists p ON i.playlist = p.id WHERE i.id = $1 AND p.id = $2", c.Query("pivot"), id).Scan(&stateSecret, &currentIndex)
	if err == sql.ErrNoRows {
		fail("Playlist not found. Please refresh the page.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	pivot, pivotIndex, err := getPivot(c, stateSecret)
	if err != nil {
		fail("", err)
		return
	}

	i := 0
	nextId := ds.Nilable(pivot)
	for ; i < limit && nextId.Valid; i++ {
		item, err := queryItem(tx, nextId.UUID, i+pivotIndex, stateSecret)
		if err != nil {
			fail("", err)
			return
		}

		items = append(items, item)
		nextId = item.Next
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	slices.Reverse(items)

	args := gin.H{
		"Id":         id,
		"GlobalHash": hashSecret(stateSecret),
		"Items":      items,
		"ThisPage":   pivotArgs(items[len(items)-1].Id, items[len(items)-1].Index, stateSecret),
		"CurrentIdx": currentIndex,
	}
	if items[len(items)-1].Prev.Valid {
		args["PrevPage"] = pivotArgs(items[len(items)-1].Prev.UUID, items[len(items)-1].Index-1, stateSecret)
	}
	if nextId.Valid {
		args["NextPage"] = pivotArgs(nextId.UUID, i+pivotIndex, stateSecret)
	}
	SSR(playlistWatchTmpl, c, "queue", args)
}

func playlistWatchQueuePrevPage(c *gin.Context) {
	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist previous page fetch error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		playlistWatchQueueFirstPage(c)
		Toast(c, ToastError, "Unable to fetch playlist queue", msg)
	}

	id := c.Param("id")
	limit := queueLimit(c)

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var items []QueuePlaylistItem
	var stateSecret string
	var currentIndex int
	err = tx.QueryRow("SELECT p.state_secret, p.current_idx FROM playlist_items i INNER JOIN playlists p ON i.playlist = p.id WHERE i.id = $1 AND p.id = $2", c.Query("pivot"), id).Scan(&stateSecret, &currentIndex)
	if err == sql.ErrNoRows {
		fail("Playlist not found. Please refresh the page.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	pivot, pivotIndex, err := getPivot(c, stateSecret)
	if err != nil {
		fail("", err)
		return
	}

	i := 0
	prevId := ds.Nilable(pivot)
	for ; i < limit && prevId.Valid; i++ {
		item, err := queryItem(tx, prevId.UUID, pivotIndex-i, stateSecret)
		if err != nil {
			fail("", err)
			return
		}

		items = append(items, item)
		prevId = item.Prev
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	args := gin.H{
		"Id":         id,
		"GlobalHash": hashSecret(stateSecret),
		"Items":      items,
		"ThisPage":   pivotArgs(items[len(items)-1].Id, items[len(items)-1].Index, stateSecret),
		"CurrentIdx": currentIndex,
	}
	if items[0].Next.Valid {
		args["NextPage"] = pivotArgs(items[0].Next.UUID, items[0].Index+1, stateSecret)
	}
	if prevId.Valid {
		args["PrevPage"] = pivotArgs(prevId.UUID, pivotIndex-i, stateSecret)
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
	var stateSecret string
	var first uuid.NullUUID
	var last uuid.NullUUID
	err = tx.QueryRow("SELECT name, owner_username, created_timestamp, state_secret, first, last FROM playlists WHERE id = $1", id).Scan(&name, &owner, &createdTimestamp, &stateSecret, &first, &last)
	if err != nil {
		fail("", err)
		return
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	globalHash := hashSecret(stateSecret)

	SSR(playlistWatchTmpl, c, "controller", gin.H{
		"Id":               id,
		"Name":             name,
		"Owner":            owner,
		"CreatedTimestamp": createdTimestamp,
		"GlobalHash":       globalHash,
		"Media": gin.H{
			"Id":             "884ae4e9-cea4-4c49-a19d-5e5cd7b3820f",
			"ItemId":         "884ae4e9-cea4-4c49-a19d-5e5cd7b3820f",
			"PrevId":         "884ae4e9-cea4-4c49-a19d-5e5cd7b3820f",
			"NextId":         "none",
			"Type":           "yt",
			"URL":            "https://youtu.be/FrcR9qvjwmo",
			"Title":          "Hello, World!",
			"Artist":         "Kizuna Ai",
			"OriginalTitle":  "hello world",
			"OriginalArtist": "Kizuna Ai",
			"Duration":       time.Duration(100) * time.Second,
		},
	})
}

func updatePlaylist(tx *sql.Tx, id string) (string, error) {
	secret := uuid.New().String()
	_, err := tx.Exec("UPDATE playlists SET state_secret = $1 WHERE id = $2", secret, id)
	return secret, err
}

func playlistAdd(c *gin.Context) {
	id := c.Param("id")
	username, loggedIn := middlewares.GetAuthUsername(c)
	pos := c.PostForm("add-position")
	title := c.PostForm("title")
	artist := c.PostForm("artist")
	stateHash := c.PostForm("state-hash")

	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist item add error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		playlistWatchQueueFirstPage(c)
		Toast(c, ToastError, "Unable to add media to playlist", msg)
	}

	if !loggedIn {
		fail("You must be logged in to add media to this playlist.", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	row := tx.QueryRow("SELECT state_secret FROM playlists WHERE id = $1 AND owner_username = $2", id, username)
	var secret string
	err = row.Scan(&secret)
	if err == sql.ErrNoRows {
		fail("You must be a manager of this playlist to modify its content.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	checksum := hashSecret(secret)
	if stateHash != checksum {
		fail("Invalid playlist state hash", err)
		return
	}

	secret, err = updatePlaylist(tx, id)
	if err != nil {
		fail("", err)
		return
	}

	if pos != string(ds.AddToStart) && pos != string(ds.AddToEnd) && pos != string(ds.QueueNext) {
		fail("Invalid add position", nil)
		return
	}

	playlist := ds.CreatePlaylistWrapper(id, tx)
	var mediaId uuid.UUID
	err = tx.QueryRow("INSERT INTO medias (title, artist) VALUES ($1, $2) RETURNING id", title, artist).Scan(&mediaId)
	if err != nil {
		fail("", err)
		return
	}

	var itemId uuid.UUID
	err = tx.QueryRow("INSERT INTO playlist_items (media, playlist) VALUES ($1, $2) RETURNING id", mediaId, id).Scan(&itemId)
	if err != nil {
		fail("", err)
		return
	}

	err = ds.AddItem(playlist, itemId, ds.AddPosition(pos))
	if err != nil {
		fail("", err)
		return
	}

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	// injection vulnerability
	Toast(c, ToastInfo, "Media added successfully", template.HTML(fmt.Sprintf("Added %s - %s to playlist %s", artist, title, "")))
	playlistWatchQueueFirstPage(c)
}

func playlistGoto(c *gin.Context) {
	id := c.Param("id")

	fail := func(msg template.HTML, err error) {
		slog.Warn("Playlist goto error", "msg", msg, "err", err)
		if len(msg) == 0 {
			msg = "Internal server error. Please try again later."
		}
		playlistWatchQueueFirstPage(c)
		Toast(c, ToastError, "Unable to change current item in playlist", msg)
	}

	username, loggedIn := middlewares.GetAuthUsername(c)
	if !loggedIn {
		fail("You must be logged in to set current media of this playlist.", nil)
		return
	}

	itemId, err := uuid.Parse(c.Param("item-id"))
	if err != nil {
		fail("Invalid item UUID", err)
		return
	}

	index, err := strconv.Atoi(c.PostForm("index"))
	if err != nil {
		fail("Invalid item index", err)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	var stateSecret string
	err = tx.QueryRow("SELECT state_secret FROM playlists WHERE id = $1 AND owner_username = $2", id, username).Scan(&stateSecret)
	if err == sql.ErrNoRows {
		fail("You must be a manager of this playlist to modify its content.", err)
		return
	} else if err != nil {
		fail("", err)
		return
	}

	var dummy int
	err = tx.QueryRow("SELECT 1 FROM playlist_items WHERE id = $1 AND playlist = $2", itemId, id).Scan(&dummy)
	if err != nil {
		fail("Item does not belong to playlist", err)
		return
	}

	if c.PostForm("state-hash") != hashSecret(stateSecret) {
		fail("State hash mismatch", nil)
		return
	}

	if c.PostForm("playlist-item-hash-"+itemId.String()) != positionalHash(stateSecret, itemId, index) {
		fail("Positional hash mismatch", nil)
		return
	}

	playlist := ds.CreatePlaylistWrapper(id, tx)
	err = playlist.SetCurrent(ds.Nilable(itemId), index)
	if err != nil {
		fail("", err)
		return
	}

	stateSecret, err = updatePlaylist(tx, id)

	if err := tx.Commit(); err != nil {
		fail("", err)
		return
	}

	playlistWatchQueueFirstPage(c)
}
