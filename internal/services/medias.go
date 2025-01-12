package services

import (
	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/media"
)

func GetMediaId(tx *db.Tx, url string) (id int, hasRow, hasErr bool) {
	hasErr = tx.QueryRow("SELECT id FROM medias WHERE url = $1", url).Scan(&hasRow, &id)
	return id, hasRow, hasErr
}

func GetMediaSimpleInfo(tx *db.Tx, url string) (id int, title, artist string, hasRow, hasErr bool) {
	hasErr = tx.QueryRow("SELECT id, title, artist FROM medias WHERE url = $1", url).Scan(&hasRow, &id, &title, &artist)
	return id, title, artist, hasRow, hasErr
}

func AddMedia(tx *db.Tx, entry media.MediaListEntry) (id int, hasErr bool) {
	hasErr = tx.QueryRow("INSERT INTO medias (media_type, title, artist, duration, url, aspect_ratio) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		string(entry.CanonInfo.Kind), entry.ResolveInfo.Title, entry.ResolveInfo.Artist, int(entry.ResolveInfo.Duration.Seconds()),
		entry.CanonInfo.Url, entry.ResolveInfo.AspectRatio).Scan(nil, &id)
	return id, hasErr
}

func SetMediaAltMetadata(tx *db.Tx, title, artist string, playlist, media int) (hasErr bool) {
	return tx.Exec(nil, "INSERT INTO alt_metadata (playlist, media, alt_title, alt_artist) VALUES ($1, $2, $3, $4) ON CONFLICT (playlist, media) DO UPDATE SET alt_title = excluded.alt_title, alt_artist = excluded.alt_artist", playlist, media, title, artist)
}

func NotifyMediaChanged(tx *db.Tx, playlist int, socketId string) (callback func(), hasErr bool) {
	var payload MediaChangedPayload
	var hasRow bool
	if tx.QueryRow("SELECT m.media_type, m.url, m.aspect_ratio FROM playlists p JOIN playlist_items i ON p.current = i.id JOIN medias m ON m.id = i.media WHERE p.id = $1", playlist).Scan(&hasRow, &payload.Type, &payload.Url, &payload.AspectRatio) {
		return nil, true
	}

	if !hasRow {
		payload.Type = media.MediaKindNone
	}

	return func() {
		WebSocketMediaChange(playlist, socketId, payload)
	}, false
}
