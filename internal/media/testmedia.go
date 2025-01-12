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

	ffprobe "gopkg.in/vansante/go-ffprobe.v2"
)

func TestCanonicalizeMedia(u *url.URL) (info *MediaCanonicalizeInfo, ok bool) {
	slog.Debug("Checking if URL is a test URL or not...", "url", u)
	if u.Hostname() != "localhost" {
		slog.Debug("URL is not a localhost URL", "url", u)
		return nil, false
	}

	mediaPath, ok := strings.CutPrefix(u.Path, "/testmedias/")
	if !ok {
		slog.Debug("URL path does not start with /testmedias/", "url", u)
		return nil, false
	}

	info = &MediaCanonicalizeInfo{
		Url:      mediaPathToCanonInfo(mediaPath).Url,
		Id:       mediaPath,
		Multiple: strings.HasSuffix(mediaPath, ".json"),
	}

	if strings.HasSuffix(mediaPath, ".mp4") || strings.HasSuffix(mediaPath, ".mp4.json") {
		info.Kind = MediaKindTestVideo
	} else if strings.HasSuffix(mediaPath, ".mp3") || strings.HasSuffix(mediaPath, ".mp3.json") {
		info.Kind = MediaKindTestVideo
	} else {
		return nil, false
	}

	return info, true
}

func mediaPathToCanonInfo(path string) *MediaCanonicalizeInfo {
	// port num doesn't matter
	canonUrl, err := url.Parse("http://localhost:6972")
	if err != nil {
		panic(err)
	}

	canonUrl.Path = filepath.Join("testmedias", path)
	kind := MediaKindTestVideo
	if strings.HasSuffix(path, ".mp3") || strings.HasSuffix(path, ".mp3.json") {
		kind = MediaKindTestAudio
	}
	return &MediaCanonicalizeInfo{
		Kind:     kind,
		Url:      canonUrl.String(),
		Id:       path,
		Multiple: strings.HasSuffix(path, ".json"),
	}
}

func safeJoin(basePath, subPath string) (string, error) {
	// Clean the base path
	basePath = filepath.Clean(basePath)

	// Join the paths
	joinedPath := filepath.Join(basePath, subPath)

	// Ensure the resulting path is within the base directory
	realBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute base path: %w", err)
	}

	realJoinedPath, err := filepath.Abs(joinedPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute joined path: %w", err)
	}

	// todo: replace this
	if !filepath.HasPrefix(realJoinedPath, realBasePath) {
		return "", errors.New("resulting path is outside the base directory")
	}

	return realJoinedPath, nil
}

func TestResolveMedia(ctx context.Context, mediaUrl string) (*MediaResolveInfo, error) {
	slog.Debug("Resolving media", "url", mediaUrl)
	u, err := url.Parse(mediaUrl)
	if err != nil {
		panic(err)
	}

	mediaPath, ok := strings.CutPrefix(u.Path, "/testmedias/")
	if !ok {
		slog.Debug("URL path does not start with /testmedias/", "url", u)
		return nil, errors.New("URL path does not start with /testmedias/")
	}

	mediaPath, err = safeJoin("./www/testmedias", mediaPath)
	if err != nil {
		return nil, err
	}

	slog.Debug("Resolving media with ffprobe", "path", mediaPath)
	info, err := ffprobe.ProbeURL(ctx, mediaPath)
	if err != nil {
		return nil, err
	}

	slog.Debug("Getting media metadata...")
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

	return &MediaResolveInfo{Title: title, Artist: artist, Duration: duration, AspectRatio: aspectRatio}, nil
}

type TestMediaList struct {
	title      string
	artist     string
	mediaPaths []string
}

func TestResolveMediaList(ctx context.Context, mediaUrl string) (*MediaListResolveInfo, error) {
	slog.Debug("Resolving media", "url", mediaUrl)
	u, err := url.Parse(mediaUrl)
	if err != nil {
		panic(err)
	}

	mediaPath, ok := strings.CutPrefix(u.Path, "/testmedias/")
	if !ok {
		slog.Debug("URL path does not start with /testmedias/", "url", u)
		return nil, errors.New("URL path does not start with /testmedias/")
	}

	mediaPath, err = safeJoin("./www/testmedias", mediaPath)
	if err != nil {
		return nil, err
	}

	slog.Debug("Reading media list JSON file", "path", mediaPath)
	content, err := os.ReadFile(mediaPath)
	if err != nil {
		return nil, err
	}
	var mediaList TestMediaList
	json.Unmarshal(content, &mediaList)

	info := &MediaListResolveInfo{
		Title:  mediaList.title,
		Artist: mediaList.artist,
		Medias: []MediaListEntry{},
	}

	for _, path := range mediaList.mediaPaths {
		slog.Debug("Resolving media", slog.String("path", path))
		canonInfo := mediaPathToCanonInfo(path)
		resolveInfo, err := TestResolveMedia(ctx, canonInfo.Url)
		if err != nil {
			return nil, err
		}
		info.Medias = append(info.Medias, MediaListEntry{CanonInfo: canonInfo, ResolveInfo: resolveInfo})
	}

	return info, nil
}
