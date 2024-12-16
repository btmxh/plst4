package media

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/wader/goutubedl"
)

type YoutubeURLType int

const (
	InvalidURL  YoutubeURLType = 0
	VideoURL    YoutubeURLType = 1
	PlaylistURL YoutubeURLType = 2
)

type YoutubeURLParseResult struct {
	id string
}

func checkId(s string) bool {
	for _, r := range s {
		suitable := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !suitable {
			return false
		}
	}

	return true
}

func checkVideoId(id string) bool {
	return len(id) == 11 && checkId(id)
}

func checkPlaylistId(id string) bool {
	return checkId(id)
}

func videoInfo(id string) *MediaCanonicalizeInfo {
	return &MediaCanonicalizeInfo{Kind: MediaKindYoutube, Id: id, Url: fmt.Sprintf("https://youtu.be/%s", id), Multiple: false}
}

func playlistInfo(id string) *MediaCanonicalizeInfo {
	return &MediaCanonicalizeInfo{Kind: MediaKindYoutube, Id: id, Url: fmt.Sprintf("https://youtube.com/playlist?list=%s", id), Multiple: true}
}

func YTCanonicalizeMedia(u *url.URL) (*MediaCanonicalizeInfo, bool) {
	slog.Debug("Checking if URL is a YouTube URL or not...", "url", u)
	if u.Scheme != "https" {
		return nil, false
	}

	path, hasLeadingSlash := strings.CutPrefix(u.Path, "/")
	if !hasLeadingSlash {
		slog.Debug("Path has no leading slash, aborting...", "path", path)
		return nil, false
	}

	slog.Debug("Path has leading slash, continuing...", "path", path)

	host := u.Hostname()
	slog.Debug("Case: short video URL")
	slog.Debug("Checking hostname...", "host", host)
	slog.Debug("Checking video id...", "path", path)
	if checkVideoId(path) && (host == "yt.be" || host == "youtu.be") {
		slog.Debug("YouTube short video URL successfully recognized")
		return videoInfo(path), true
	}

	slog.Debug("Case: long video/playlist URL")
	if host == "www.youtube.com" || host == "youtube.com" {
		if id, ok := strings.CutPrefix(path, "shorts/"); ok {
			slog.Debug("path is shorts/, checking video id", "id", path)
			if checkVideoId(id) {
				slog.Debug("YouTube (shorts) long video URL successfully recognized")
				return videoInfo(id), true
			}
		}
		if path == "watch" && u.Query().Has("v") {
			slog.Debug("path is watch, checking video id", "id", u.Query().Get("v"))
			if id := u.Query().Get("v"); checkVideoId(id) {
				slog.Debug("YouTube long video URL successfully recognized")
				return videoInfo(id), true
			}
		}

		if path == "playlist" && u.Query().Has("list") {
			if id := u.Query().Get("list"); checkPlaylistId(id) {
				slog.Debug("YouTube playlist URL successfully recognized")
				return playlistInfo(id), true
			}
		}
	}

	return nil, false
}

func trimInfo(info *goutubedl.Info) {
	info.Formats = nil
	info.Subtitles = nil
}

func mediaInfoFromYT(info *goutubedl.Info) *MediaResolveInfo {
	title := info.Title
	artist := info.Artist
	if strings.TrimSpace(artist) == "" {
		artist = info.Channel
	}
	duration := time.Duration(info.Duration * float64(time.Second)).Round(time.Second)

	trimInfo(info)
	metadata, err := json.Marshal(info)
	if err != nil {
		slog.Warn("Unable to marshal YouTube metadata", "err", err)
		metadata = nil
	}

	return &MediaResolveInfo{Title: title, Artist: artist, Duration: duration, Metadata: metadata}
}

func YTResolveMedia(ctx context.Context, url string) (*MediaResolveInfo, error) {
	slog.Debug("Resolving media", "url", url)
	result, err := goutubedl.New(ctx, url, goutubedl.Options{})
	if err != nil {
		return nil, err
	}

	return mediaInfoFromYT(&result.Info), nil
}

func YTResolveMediaList(ctx context.Context, url string) (*MediaListResolveInfo, error) {
	result, err := goutubedl.New(ctx, url, goutubedl.Options{})
	if err != nil {
		return nil, err
	}

	entries := result.Info.Entries
	mediaInfo := mediaInfoFromYT(&result.Info)
	listInfo := &MediaListResolveInfo{Title: mediaInfo.Title, Artist: mediaInfo.Artist}
	for _, entry := range entries {
		listInfo.Medias = append(listInfo.Medias, MediaListEntry{CanonInfo: videoInfo(entry.ID), ResolveInfo: mediaInfoFromYT(&entry)})
	}

	return listInfo, nil
}
