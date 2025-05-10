package services

import (
	"context"
	"net/url"
	"time"

	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/media"
)

func GetMediaId(tx *db.Tx, url string) (id int, hasRow, hasErr bool) {
	hasErr = tx.QueryRow("SELECT id FROM medias WHERE url = $1", url).Scan(&hasRow, &id)
	return id, hasRow, hasErr
}

type DatabaseResolvedMediaObject struct {
	kind        media.MediaKind
	url         string
	title       string
	artist      string
	length      time.Duration
	aspectRatio string
}

func (o *DatabaseResolvedMediaObject) Kind() media.MediaKind {
	return o.kind
}

func (o *DatabaseResolvedMediaObject) Canonicalize(_ context.Context) (media.CanonicalizedMediaObject, error) {
	return o, nil
}

func (o *DatabaseResolvedMediaObject) URL() *url.URL {
	u, err := url.Parse(o.url)
	if err != nil {
		panic(err)
	}

	return u
}

func (o *DatabaseResolvedMediaObject) Resolve(_ context.Context) (media.ResolvedMediaObject, error) {
	return o, nil
}

func (o *DatabaseResolvedMediaObject) Title() string {
	return o.title
}

func (o *DatabaseResolvedMediaObject) Artist() string {
	return o.artist
}

func (o *DatabaseResolvedMediaObject) ChildEntries() []media.ResolvedMediaObjectSingle {
	return nil
}

func (o *DatabaseResolvedMediaObject) Duration() time.Duration {
	return o.length
}

func (o *DatabaseResolvedMediaObject) AspectRatio() string {
	return o.aspectRatio
}

func GetResolvedMedia(tx *db.Tx, url string) (m media.ResolvedMediaObjectSingle, hasRow, hasErr bool) {
	var obj DatabaseResolvedMediaObject
	obj.url = url
	hasErr = tx.QueryRow("SELECT media_type, title, artist, duration, aspect_ratio FROM medias WHERE url = $1", url).Scan(&hasRow, &obj.kind, &obj.title, &obj.artist, &obj.length, &obj.aspectRatio)
	return &obj, hasRow, hasErr
}

func GetMediaSimpleInfo(tx *db.Tx, url string) (id int, title, artist string, hasRow, hasErr bool) {
	hasErr = tx.QueryRow("SELECT id, title, artist FROM medias WHERE url = $1", url).Scan(&hasRow, &id, &title, &artist)
	return id, title, artist, hasRow, hasErr
}

func AddMedia(tx *db.Tx, entry media.ResolvedMediaObjectSingle) (id int, hasErr bool) {
	hasErr = tx.QueryRow(`
		WITH ins AS
		(INSERT INTO medias (media_type, title, artist, duration, url, aspect_ratio) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (url) DO NOTHING RETURNING id)
		SELECT id FROM ins
		UNION ALL
    SELECT id FROM medias WHERE url = $5
		LIMIT 1`,
		string(entry.Kind()), entry.Title(), entry.Artist(), int(entry.Duration().Seconds()), entry.URL().String(), entry.AspectRatio()).Scan(nil, &id)
	return id, hasErr
}

func SetMediaAltMetadata(tx *db.Tx, title, artist string, playlist, media int) (hasErr bool) {
	return tx.Exec(nil, "INSERT INTO alt_metadata (playlist, media, alt_title, alt_artist) VALUES ($1, $2, $3, $4) ON CONFLICT (playlist, media) DO UPDATE SET alt_title = excluded.alt_title, alt_artist = excluded.alt_artist", playlist, media, title, artist)
}

func NotifyMediaChanged(tx *db.Tx, playlist int, socketId string) (callback func(), hasErr bool) {
	var payload MediaChangedPayload
	var hasRow bool
	if tx.QueryRow("SELECT m.media_type, m.url, m.aspect_ratio, p.current_version FROM playlists p JOIN playlist_items i ON p.current = i.id JOIN medias m ON m.id = i.media WHERE p.id = $1", playlist).Scan(&hasRow, &payload.Type, &payload.Url, &payload.AspectRatio, &payload.NewVersion) {
		return nil, true
	}

	if !hasRow {
		payload.Type = media.MediaKindNone
	}

	return func() {
		WebSocketMediaChange(playlist, socketId, payload)
	}, false
}
