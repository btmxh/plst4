package routes

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"html/template"
	"log/slog"
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
var playlistWatchTmpl = getTemplate("watch", "templates/playlists/watch.tmpl")
var playlistWatchInvalidTmpl = getTemplate("watch", "templates/playlists/watch_invalid.tmpl")
var playlistQueryResult = getTemplate("playlist_query_result", "templates/playlists/playlist_query_result.tmpl")

func WatchRouter(g *gin.RouterGroup) {
	g.GET("/", SSRRoute(watchTemplate, "layout", gin.H{}))
	g.GET("/:id/", playlistWatchPageRouter("layout"))
	g.GET("/:id/controller", playlistWatchPageRouter("controller"))
	g.PATCH("/:id/controller/rename", func(c *gin.Context) {
		renamePlaylist(c)
		playlistWatchPageRouter("controller")(c)
	})
	g.DELETE("/:id/controller/delete", func(c *gin.Context) {
		deletePlaylist(c)
		Redirect(c, "/watch")
	})
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
		fail("You must be logged in to do this", nil)
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

func playlistWatchPageRouter(block string) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		err = tx.QueryRow("SELECT name, owner_username, created_timestamp, state_secret FROM playlists WHERE id = $1", id).Scan(&name, &owner, &createdTimestamp, &stateSecret)
		if err != nil {
			fail("", err)
			return
		}

		if err := tx.Commit(); err != nil {
			fail("", err)
			return
		}

		hash := sha256.Sum256([]byte(stateSecret))
		globalHash := base64.StdEncoding.EncodeToString(hash[:])

		SSR(playlistWatchTmpl, c, block, gin.H{
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
}
