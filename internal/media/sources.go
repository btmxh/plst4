package media

import (
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
)

var mediaSources []MediaSource

func InitMediaSources() {
	ytApiKey := os.Getenv("YOUTUBE_API_KEY")
	if ytApiKey != "" {
		mediaSources = append(mediaSources, NewYoutubeAPI(ytApiKey))
	}

	mediaSources = append(mediaSources, NewYoutubeDL())

	if gin.IsDebugging() {
		mediaSources = append(mediaSources, NewTestMediaResolver())
	}
}

func ProcessURL(u string) (MediaObject, error) {
	mediaUrl, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	for _, source := range mediaSources {
		m, err := source.ProcessURL(mediaUrl)
		if err == ErrUnsupportedURL {
			continue
		}

		return m, err
	}

	return nil, ErrUnsupportedURL
}
