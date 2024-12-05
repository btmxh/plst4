package routes

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/btmxh/plst4/internal/db"
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

const (
	All     PlaylistFilter = "all"
	Owned   PlaylistFilter = "owned"
	Managed PlaylistFilter = "managed"
)

var watchTemplate = getTemplate("watch", "templates/watch.tmpl")
var playlistQueryResult = getTemplate("playlist_query_result", "templates/playlists/playlist_query_result.tmpl")

func WatchRouter(g *gin.RouterGroup) {
	g.GET("/", SSRRoute(watchTemplate, "layout", gin.H{}))
	g.GET("/:id/")
}

func PlaylistRouter(g *gin.RouterGroup) {
	g.GET("/search", search)
	g.POST("/new", newPlaylist)
	g.PATCH("/:id/rename", renamePlaylist)
	g.DELETE("/:id/delete", deletePlaylist)
}

func search(c *gin.Context) {
	username, loggedIn := middlewares.GetAuthUsername(c)
	if !loggedIn {
		username = ""
	}

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

	c.Redirect(http.StatusSeeOther, "/watch/"+id.String())
}

func renamePlaylist(c *gin.Context) {
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
		return
	}

	if len(name) == 0 {
		fail("Playlist name must not be empty", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	row, err := tx.Exec("UPDATE playlists SET name = $1 WHERE id = $2 AND owner_username = $3", name, id, username)
	if err != nil {
		fail("", err)
		return
	}

	if affected, err := row.RowsAffected(); affected == 0 || err != nil {
		fail("", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
		return
	}

	Refresh(c)
}

func deletePlaylist(c *gin.Context) {
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
		fail("You must be logged in to do this", nil)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		fail("", err)
		return
	}
	defer tx.Rollback()

	row, err := tx.Exec("DELETE FROM playlists WHERE id = $1 AND owner_username = $2", id, username)
	if err != nil {
		fail("", err)
		return
	}

	if affected, err := row.RowsAffected(); affected == 0 || err != nil {
		fail("", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		fail("", err)
		return
	}

	Refresh(c)
}
