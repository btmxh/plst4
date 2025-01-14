package media

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const UnknownTitle string = "<unknown title>"
const UnknownArtist string = "<unknown artist>"

type MediaResolveInfo struct {
	Title       string
	Artist      string
	Duration    time.Duration
	AspectRatio string
	Metadata    []byte
}

type MediaListEntry struct {
	CanonInfo   *MediaCanonicalizeInfo
	ResolveInfo *MediaResolveInfo
}

type MediaListResolveInfo struct {
	Title  string
	Artist string
	Medias []MediaListEntry
}

func ResolveMedia(ctx context.Context, info *MediaCanonicalizeInfo) (*MediaResolveInfo, error) {
	switch info.Kind {
	case MediaKindYoutube:
		return YTResolveMedia(ctx, info.Url)
	case MediaKindTestVideo:
		return TestResolveMedia(ctx, info.Url)
	case MediaKindTestAudio:
		return TestResolveMedia(ctx, info.Url)
	}

	slog.Warn("reach here?", "info", info)
	return nil, fmt.Errorf("Unsupported media kind")
}

func ResolveMediaList(ctx context.Context, info *MediaCanonicalizeInfo) (*MediaListResolveInfo, error) {
	switch info.Kind {
	case MediaKindYoutube:
		return YTDLResolveMediaList(ctx, info.Url)
	}

	return nil, fmt.Errorf("Unsupported media kind")
}
