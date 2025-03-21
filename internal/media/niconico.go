package media

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/wader/goutubedl"
)

const MediaKindNiconico MediaKind = "2525"

var ErrInvalidNiconicoURL = errors.New("Invalid Niconico URL")

func NewNiconicoYtdlResolver() *YtdlResolver {
	return &YtdlResolver{
		mediaUrlPattern:     "https://www.nicovideo.jp/watch/%s",
		mediaListUrlPattern: "",
		searchPrefix:        "nicosearch1:",
		// this is not really used as media lists are currently unsupported
		idExtractor: func(info goutubedl.Info) string {
			id := strings.TrimPrefix(info.WebpageURL, "https://www.nicovideo.jp/watch/")
			if id == info.WebpageURL {
				panic("incomplete niconico info")
			}

			return id
		},
	}
}

type NiconicoSource struct {
	resolver *YtdlResolver
}

func NewNiconico() *NiconicoSource {
	return &NiconicoSource{resolver: NewNiconicoYtdlResolver()}
}

func (nc *NiconicoSource) Kind() MediaKind {
	return MediaKindNiconico
}

func (nc *NiconicoSource) MediaURL(id string) *url.URL {
	return nc.resolver.MediaURL(id)
}

func (nc *NiconicoSource) MediaListURL(id string) *url.URL {
	panic("unsupported")
}

func (nc *NiconicoSource) ResolveMedia(ctx context.Context, id string) (ResolvedMediaObjectSingle, error) {
	return nc.resolver.ResolveMedia(nc, ctx, id)
}

func (nc *NiconicoSource) ResolveMediaList(ctx context.Context, id string) (ResolvedMediaObject, error) {
	panic("unsupported")
}

func (nc *NiconicoSource) ProcessURL(u *url.URL) (MediaObject, error) {
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, ErrUnsupportedURL
	}

	path := u.Path

	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	if u.Hostname() != "nicovideo.jp" && u.Hostname() != "www.nicovideo.jp" {
		return nil, ErrUnsupportedURL
	}

	if id, found := strings.CutPrefix(path, "watch/"); found {
		return NewIdMediaObject(nc, id, nil), nil
	}

	return nil, ErrInvalidNiconicoURL
}
