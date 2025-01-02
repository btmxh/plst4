package media

import (
	"fmt"
	"net/url"
)

type MediaKind string

const (
	MediaKindNone    MediaKind = "none"
	MediaKindYoutube MediaKind = "yt"
)

type MediaCanonicalizeInfo struct {
	Kind     MediaKind
	Id       string
	Url      string
	Multiple bool
}

func CanonicalizeMedia(mediaUrl string) (*MediaCanonicalizeInfo, error) {
	u, err := url.Parse(mediaUrl)
	if err != nil {
		return nil, err
	}

	if info, ok := YTCanonicalizeMedia(u); ok {
		return info, nil
	}

	return nil, fmt.Errorf("URL not supported")
}
