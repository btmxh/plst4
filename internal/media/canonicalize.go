package media

import (
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"
)

type MediaKind string

const (
	MediaKindNone      MediaKind = "none"
	MediaKindYoutube   MediaKind = "yt"
	MediaKindTestVideo MediaKind = "testvideo"
	MediaKindTestAudio MediaKind = "testaudio"
)

type MediaCanonicalizeInfo struct {
	Kind     MediaKind
	Id       string
	Url      string
	Multiple bool
}

func getEnabledMediaKinds() map[MediaKind]struct{} {
	kinds := make(map[MediaKind]struct{})
	kinds[MediaKindYoutube] = struct{}{}
	if gin.IsDebugging() {
		kinds[MediaKindTestVideo] = struct{}{}
		kinds[MediaKindTestAudio] = struct{}{}
	}
	return kinds
}

var EnabledMediaKinds = getEnabledMediaKinds()

func CanonicalizeMedia(mediaUrl string) (*MediaCanonicalizeInfo, error) {
	u, err := url.Parse(mediaUrl)
	if err != nil {
		return nil, err
	}

	if _, ok := EnabledMediaKinds[MediaKindYoutube]; ok {
		if info, ok := YTCanonicalizeMedia(u); ok {
			return info, nil
		}
	}

	_, okVideo := EnabledMediaKinds[MediaKindTestVideo]
	_, okAudio := EnabledMediaKinds[MediaKindTestAudio]
	if okVideo || okAudio {
		if info, ok := TestCanonicalizeMedia(u); ok {
			return info, nil
		}
	}

	return nil, fmt.Errorf("URL not supported")
}
