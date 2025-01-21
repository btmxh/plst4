package services

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/errs"
)

type PlaylistAddPosition string

const (
	AddToStart PlaylistAddPosition = "add-to-start"
	AddToEnd   PlaylistAddPosition = "add-to-end"
	QueueNext  PlaylistAddPosition = "queue-next"
)

func ParsePlaylistAddPosition(pos string) (PlaylistAddPosition, error) {
	switch pos {
	case string(AddToStart), string(AddToEnd), string(QueueNext):
		return PlaylistAddPosition(pos), nil
	default:
		return "", fmt.Errorf("Invalid add position: %s", pos)
	}
}

const PlaylistAddOrderGap = 1 << 10

type PlaylistFilter string

const (
	All     PlaylistFilter = "all"
	Owned   PlaylistFilter = "owned"
	Managed PlaylistFilter = "managed"
)

var NoCurrentMediaError = errors.New("No current media.")
var AlreadyPlaylistOwnerError = errors.New("This user (you) is already the playlist owner.")
var AlreadyPlaylistManagerError = errors.New("This user is already a playlist manager.")
var UserNotFoundError = errors.New("User not found.")

func ParsePlaylistFilter(filter string) (PlaylistFilter, error) {
	switch filter {
	case string(All), string(Owned), string(Managed):
		return PlaylistFilter(filter), nil
	default:
		return "", fmt.Errorf("Invalid filter: %s", filter)
	}
}

type QueriedPlaylist struct {
	Id               int
	Name             string
	OwnerUsername    string
	CreatedTimestamp time.Time
	ItemCount        int
	TotalLength      time.Duration
	CurrentPlaying   string
}

func IsPlaylistOwner(tx *db.Tx, username string, playlist int) (isOwner, hasErr bool) {
	var dummy int
	var hasRow bool
	hasErr = tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1 AND owner_username = $2", playlist, username).Scan(&hasRow, &dummy)
	return hasRow, hasErr
}

func IsPlaylistManager(tx *db.Tx, username string, playlist int) (isManager, hasError bool) {
	isOwner, hasErr := IsPlaylistOwner(tx, username, playlist)
	if isOwner || hasErr {
		return isOwner, hasErr
	}

	var dummy int
	if tx.QueryRow("SELECT 1 FROM playlist_manage WHERE playlist = $1 AND username = $2", playlist, username).Scan(&isManager, &dummy) {
		return false, true
	}

	return isManager, false
}

func SearchPlaylists(tx *db.Tx, username string, query string, filter PlaylistFilter, offset int) (page Pagination[QueriedPlaylist], hasError bool) {
	var rows *sql.Rows
	var hasErr bool
	switch filter {
	case All:
		hasErr = tx.Query(&rows,
			`SELECT id, name, owner_username, created_timestamp, 
              COALESCE((SELECT COUNT(*) FROM playlist_items WHERE playlist = playlists.id), 0),
							COALESCE((SELECT SUM(m.duration) FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = playlists.id), 0)
         FROM playlists 
         WHERE POSITION($1 IN LOWER(name)) > 0 
         ORDER BY created_timestamp DESC
         LIMIT $2 OFFSET $3`,
			query, DefaultPagingLimit+1, offset,
		)
		break
	case Owned:
		hasErr = tx.Query(&rows,
			`SELECT id, name, owner_username, created_timestamp, 
              COALESCE((SELECT COUNT(*) FROM playlist_items WHERE playlist = playlists.id), 0),
							COALESCE((SELECT SUM(m.duration) FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = playlists.id), 0)
         FROM playlists 
         WHERE POSITION($1 IN LOWER(name)) > 0 
           AND owner_username = $4 
         ORDER BY created_timestamp 
         LIMIT $2 OFFSET $3`,
			query, DefaultPagingLimit+1, offset, username,
		)
		break
	case Managed:
		hasErr = tx.Query(&rows,
			`SELECT id, name, owner_username, created_timestamp, 
              COALESCE((SELECT COUNT(*) FROM playlist_items WHERE playlist = playlists.id), 0),
							COALESCE((SELECT SUM(m.duration) FROM playlist_items i JOIN medias m ON m.id = i.media WHERE i.playlist = playlists.id), 0)
         FROM playlists 
         WHERE POSITION($1 IN LOWER(name)) > 0 
           AND (owner_username = $4 OR $4 IN (SELECT username FROM playlist_manage WHERE playlist = playlists.id))
         ORDER BY created_timestamp 
         LIMIT $2 OFFSET $3`,
			query, DefaultPagingLimit+1, offset, username,
		)
	}

	if hasErr {
		return
	}

	var playlists []QueriedPlaylist
	for rows.Next() {
		var playlist QueriedPlaylist
		var totalLength int
		if err := rows.Scan(&playlist.Id, &playlist.Name, &playlist.OwnerUsername, &playlist.CreatedTimestamp, &playlist.ItemCount, &totalLength); err != nil {
			tx.PrivateError(err)
			tx.PublicError(http.StatusInternalServerError, db.GenericError)
			return
		}

		playlist.TotalLength = time.Duration(totalLength) * time.Second
		playlists = append(playlists, playlist)
	}

	return NewPagination(offset, playlists), false
}

func CreatePlaylist(tx *db.Tx, username string, name string) (id int, hasError bool) {
	var hasRow bool
	if tx.QueryRow("INSERT INTO playlists (name, owner_username) VALUES ($1, $2) RETURNING id", name, username).Scan(&hasRow, &id) || !hasRow {
		return 0, true
	}

	return id, false
}

func RenamePlaylist(tx *db.Tx, username string, id int, name string) (hasErr bool) {
	return tx.Exec(nil, "UPDATE playlists SET name = $1 WHERE id = $2 AND owner_username = $3", name, id, username)
}

func DeletePlaylist(tx *db.Tx, username string, id int) (hasErr bool) {
	return tx.Exec(nil, "DELETE FROM playlists WHERE id = $1 AND owner_username = $2", id, username)
}

func CheckPlaylistExists(tx *db.Tx, id int) (hasRow bool, hasErr bool) {
	hasErr = tx.QueryRow("SELECT 1 FROM playlists WHERE id = $1", id).Scan(&hasRow)
	return hasRow, hasErr
}

func SetCurrentMedia(tx *db.Tx, playlist int, itemId sql.NullInt32) (hasErr bool) {
	return tx.Exec(nil, "UPDATE playlists SET current = $1 WHERE id = $2", itemId, playlist)
}

func GetCurrentMedia(tx *db.Tx, playlist int) (itemId sql.NullInt32, hasErr bool) {
	hasErr = tx.QueryRow("SELECT current FROM playlists WHERE id = $1", playlist).Scan(nil, &itemId)
	return itemId, hasErr
}

func PlaylistUpdateCurrent(tx *db.Tx, handler errs.ErrorHandler, playlist int, sign, sortOrder string) (callback func(), hasErr bool) {
	var currentOrder int

	var hasRow bool
	var current sql.NullInt32
	if tx.QueryRow("SELECT current FROM playlists WHERE id = $1", playlist).Scan(nil, &current) {
		return nil, true
	}

	if !current.Valid {
		handler.PublicError(http.StatusNotFound, NoCurrentMediaError)
		return nil, true
	}

	if tx.QueryRow("SELECT item_order FROM playlist_items WHERE id = $1", current).Scan(nil, &currentOrder) {
		return nil, true
	}

	var next int
	if tx.QueryRow("SELECT id FROM playlist_items WHERE playlist = $1 AND item_order "+sign+" $2 ORDER BY item_order "+sortOrder, playlist, currentOrder).Scan(&hasRow, &next) {
		return nil, true
	}
	if !hasRow {
		if tx.QueryRow("SELECT id FROM playlist_items WHERE playlist = $1 ORDER BY item_order "+sortOrder, playlist).Scan(nil, &next) {
			return nil, true
		}
	}

	if SetCurrentMedia(tx, playlist, sql.NullInt32{Int32: int32(next), Valid: true}) {
		return nil, true
	}

	callback, hasErr = NotifyMediaChanged(tx, playlist, "")
	return callback, hasErr
}

func SendNextRequest(tx *db.Tx, handler errs.ErrorHandler, playlist int, username string) (callback func(), hasErr bool) {
	if p, ok := manager.playlists[playlist]; ok {
		p.nextRequested[username] = struct{}{}

		for username := range p.userSockets {
			if username == "" {
				continue
			}

			if _, ok := p.nextRequested[username]; !ok {
				return func() {}, false
			}
		}

		for k := range p.nextRequested {
			delete(p.nextRequested, k)
		}

		return PlaylistUpdateCurrent(tx, handler, playlist, ">", "ASC")
	}

	return func() {}, false
}

func GetPlaylistOwner(tx *db.Tx, playlist int) (owner string, hasErr bool) {
	hasErr = tx.QueryRow("SELECT owner_username FROM playlists WHERE id = $1", playlist).Scan(nil, &owner)
	return owner, hasErr
}

// excluding owner
func EnumeratePlaylistManagers(tx *db.Tx, playlist int) (managers []string, hasErr bool) {
	var rows *sql.Rows
	if tx.Query(&rows, "SELECT username FROM playlist_manage WHERE playlist = $1", playlist) {
		return nil, true
	}

	for rows.Next() {
		var manager string
		if err := rows.Scan(&manager); err != nil {
			tx.PrivateError(err)
			tx.PublicError(http.StatusInternalServerError, db.GenericError)
			return nil, true
		}

		managers = append(managers, manager)
	}

	return managers, false
}

func AddPlaylistManager(tx *db.Tx, playlist int, username string) (hasErr bool) {
	if isOwner, hasErr := IsPlaylistOwner(tx, username, playlist); isOwner || hasErr {
		if !hasErr {
			tx.PublicError(http.StatusForbidden, AlreadyPlaylistOwnerError)
		}
		return true
	}

	if isManager, hasErr := IsPlaylistManager(tx, username, playlist); isManager || hasErr {
		if !hasErr {
			tx.PublicError(http.StatusForbidden, AlreadyPlaylistManagerError)
		}
		return hasErr
	}

	if userExists, hasErr := CheckUserExists(tx, username); !userExists || hasErr {
		if !hasErr {
			tx.PublicError(http.StatusNotFound, UserNotFoundError)
		}
		return hasErr
	}

	return tx.Exec(nil, "INSERT INTO playlist_manage (playlist, username) VALUES ($1, $2)", playlist, username)
}

func DeletePlaylistManager(tx *db.Tx, playlist int, username string) (hasErr bool) {
	return tx.Exec(nil, "DELETE FROM playlist_manage WHERE playlist = $1 AND username = $2", playlist, username)
}
