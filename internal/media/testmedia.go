package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	ffprobe "gopkg.in/vansante/go-ffprobe.v2"
)

type TestMediaResolver struct{}

func NewTestMediaResolver() *TestMediaResolver {
  return &TestMediaResolver{}
}

type TestMediaInfo struct {
	title       string
	artist      string
	length      time.Duration
	aspectRatio string
}

type TestMedia struct {
	path string
	info *TestMediaInfo
}

func newTestMedia(path string, info *TestMediaInfo) *TestMedia {
	return &TestMedia{path: path, info: info}
}

func (m *TestMedia) Kind() MediaKind {
	if strings.Contains(m.path, "mp4") {
		return MediaKindTestVideo
	} else {
		return MediaKindTestAudio
	}
}

func (m *TestMedia) Canonicalize(_ context.Context) (CanonicalizedMediaObject, error) {
	return m, nil
}

func (m *TestMedia) URL() *url.URL {
	u, err := url.Parse(os.Getenv("HOSTNAME"))
	if err != nil {
		panic(err)
	}

	u.Path = filepath.Join("testmedias", m.path)
	return u
}

func (m *TestMedia) Resolve(ctx context.Context) (ResolvedMediaObject, error) {
	if m.info != nil {
		return m, nil
	}

	return resolveSingle(ctx, m)
}

func (m *TestMedia) Title() string {
	return m.info.title
}

func (m *TestMedia) Artist() string {
	return m.info.artist
}

func (m *TestMedia) ChildEntries() []ResolvedMediaObjectSingle {
	return nil
}

func (m *TestMedia) Duration() time.Duration {
	return m.info.length
}

func (m *TestMedia) AspectRatio() string {
	return m.info.aspectRatio
}

type TestMediaListInfo struct {
	title  string
	artist string
	medias []TestMedia
}

type TestMediaList struct {
	path string
	info *TestMediaListInfo
}

func newTestMediaList(path string, info *TestMediaListInfo) *TestMediaList {
	return &TestMediaList{path: path, info: info}
}

func (m *TestMediaList) Kind() MediaKind {
	if strings.Contains(m.path, "mp4") {
		return MediaKindTestVideo
	} else {
		return MediaKindTestAudio
	}
}

func (m *TestMediaList) Canonicalize(_ context.Context) (CanonicalizedMediaObject, error) {
	return m, nil
}

func (m *TestMediaList) URL() *url.URL {
	u, err := url.Parse(os.Getenv("HOSTNAME"))
	if err != nil {
		panic(err)
	}

	u.Path = filepath.Join("testmedias", m.path)
	return u
}

func (m *TestMediaList) Resolve(ctx context.Context) (ResolvedMediaObject, error) {
	if m.info != nil {
		return m, nil
	}

	return resolveList(ctx, m)
}

func (m *TestMediaList) Title() string {
	return m.info.title
}

func (m *TestMediaList) Artist() string {
	return m.info.artist
}

func (m *TestMediaList) ChildEntries() (entries []ResolvedMediaObjectSingle) {
	for _, media := range m.info.medias {
		entries = append(entries, &media)
	}

	return entries
}

func resolveList(ctx context.Context, m *TestMediaList) (ResolvedMediaObject, error) {
	slog.Debug("Resolving test media", "url", m.path)
	mediaPath := filepath.Join("./www/testmedias", m.path)

	slog.Debug("Reading media list JSON file", "path", mediaPath)
	content, err := os.ReadFile(mediaPath)
	if err != nil {
		return nil, err
	}
	var mediaList TestMediaListJson
	json.Unmarshal(content, &mediaList)

	list := newTestMediaList(m.path,
		&TestMediaListInfo{
			title:  mediaList.Title,
			artist: mediaList.Artist,
			medias: []TestMedia{},
		})

	for _, path := range mediaList.MediaPaths {
		slog.Debug("Resolving media", slog.String("path", path))
		media, err := resolveSingle(ctx, newTestMedia(path, nil))
		if err != nil {
			return nil, err
		}

		list.info.medias = append(list.info.medias, *media)
	}

	return list, nil
}

func resolveSingle(ctx context.Context, m *TestMedia) (*TestMedia, error) {
	mediaPath := filepath.Join("./www/testmedias", m.path)

	info, err := ffprobe.ProbeURL(ctx, mediaPath)
	if err != nil {
		return nil, err
	}

	title, err := info.Format.TagList.GetString("title")
	if err != nil {
		title = UnknownTitle
	}

	artist, err := info.Format.TagList.GetString("artist")
	if err != nil {
		artist = UnknownArtist
	}

	duration := info.Format.Duration()
	aspectRatio := "16/9"
	videoStream := info.FirstVideoStream()
	if videoStream != nil {
		aspectRatio = fmt.Sprintf("%d/%d", int(videoStream.Width), int(videoStream.Height))
	}

	m.info = &TestMediaInfo{
		title:       title,
		artist:      artist,
		length:      duration,
		aspectRatio: aspectRatio,
	}

	return m, nil
}

var ErrInvalidTestMediaURL = errors.New("Invalid test media URL")

func (v *TestMediaResolver) ProcessURL(u *url.URL) (MediaObject, error) {
	slog.Debug("Checking if URL is a test URL or not...", "url", u)
	if u.Hostname() != "localhost" || u.Scheme != "http" {
		slog.Debug("URL is not a http://localhost:* URL", "url", u)
		return nil, ErrUnsupportedURL
	}

	mediaPath, ok := strings.CutPrefix(u.Path, "/testmedias/")
	if !ok {
		slog.Debug("URL path does not start with /testmedias/", "url", u)
		return nil, ErrInvalidTestMediaURL
	}

	if strings.Contains(mediaPath, "..") {
		slog.Debug("Attempting to do arbitrary path traversal", "url", u)
		return nil, ErrInvalidTestMediaURL
	}

	if strings.Contains(mediaPath, "json") {
		return newTestMediaList(mediaPath, nil), nil
	} else {
		return newTestMedia(mediaPath, nil), nil
	}
}

type TestMediaListJson struct {
	Title      string   `json:"title"`
	Artist     string   `json:"artist"`
	MediaPaths []string `json:"mediaPaths"`
}
