package media

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/wader/goutubedl"
)

const MediaKindSoundcloud MediaKind = "sc"

var ErrInvalidSCURL = errors.New("Invalid SoundCloud URL")

func NewSoundcloudYtdlResolver() *YtdlResolver {
	return &YtdlResolver{
		mediaUrlPattern:     "https://soundcloud.com/%s",
		mediaListUrlPattern: "",
		searchPrefix:        "scsearch1:",
		// this is not really used as media lists are currently unsupported
		idExtractor: func(info goutubedl.Info) string {
			id := strings.TrimPrefix(info.WebpageURL, "https://soundcloud.com/")
			if id == info.WebpageURL {
				panic("incomplete soundcloud info")
			}

			return id
		},
	}
}

type SoundcloudSource struct {
	resolver *YtdlResolver
}

func NewSoundcloud() *SoundcloudSource {
	return &SoundcloudSource{resolver: NewSoundcloudYtdlResolver()}
}

func (sc *SoundcloudSource) Kind() MediaKind {
	return MediaKindSoundcloud
}

func (sc *SoundcloudSource) MediaURL(id string) *url.URL {
	return sc.resolver.MediaURL(id)
}

func (sc *SoundcloudSource) MediaListURL(id string) *url.URL {
	panic("unsupported")
}

func (sc *SoundcloudSource) ResolveMedia(ctx context.Context, id string) (ResolvedMediaObjectSingle, error) {
	return sc.resolver.ResolveMedia(sc, ctx, id)
}

func (sc *SoundcloudSource) ResolveMediaList(ctx context.Context, id string) (ResolvedMediaObject, error) {
	panic("unsupported")
}

func (sc *SoundcloudSource) ProcessURL(u *url.URL) (MediaObject, error) {
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, ErrUnsupportedURL
	}

	path := u.Path

	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	if u.Hostname() != "soundcloud.com" && u.Hostname() != "www.soundcloud.com" {
		return nil, ErrUnsupportedURL
	}

	if len(strings.Split(path, "/")) != 2 {
		return nil, ErrInvalidSCURL
	}

	return NewIdMediaObject(sc, path, nil), nil
}
