package media

import (
	"context"
	"errors"
	"net/url"
	"time"
)

type MediaKind string

const (
	MediaKindNone      MediaKind = "none"
	MediaKindTestVideo MediaKind = "testvideo"
	MediaKindTestAudio MediaKind = "testaudio"
	UnknownTitle = "Unknown title"
	UnknownArtist = "Unknown artist"
)

var ErrUnsupportedURL = errors.New("Unsupported URL")
var ErrUnsupportedOperation = errors.New("Unsupported operation")
var ErrMediaNotFound = errors.New("Media not found")

type MediaObject interface {
	Kind() MediaKind
	Canonicalize(ctx context.Context) (CanonicalizedMediaObject, error)
}

type CanonicalizedMediaObject interface {
	MediaObject

	URL() *url.URL
	Resolve(ctx context.Context) (ResolvedMediaObject, error)
}

type ResolvedMediaObject interface {
	CanonicalizedMediaObject

	Title() string
	Artist() string
	ChildEntries() []ResolvedMediaObjectSingle
}

type ResolvedMediaObjectSingle interface {
	ResolvedMediaObject

	Duration() time.Duration
	AspectRatio() string
}

type MediaSource interface {
	ProcessURL(u *url.URL) (MediaObject, error)
}
