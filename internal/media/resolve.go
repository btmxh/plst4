package media

import (
	"context"
	"fmt"
	"time"
)

const UnknownArtist string = "<unknown artist>"

type MediaResolveInfo struct {
	Title    string
	Artist   string
	Duration time.Duration
	Metadata []byte
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
	}

	return nil, fmt.Errorf("Unsupported media kind")
}

func ResolveMediaList(ctx context.Context, info *MediaCanonicalizeInfo) (*MediaListResolveInfo, error) {
	switch info.Kind {
	case MediaKindYoutube:
		return YTResolveMediaList(ctx, info.Url)
	}

	return nil, fmt.Errorf("Unsupported media kind")
}
