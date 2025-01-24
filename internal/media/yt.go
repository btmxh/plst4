package media

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/senseyeio/duration"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func NewYoutubeYtdlResolver() *YtdlResolver {
	return &YtdlResolver{
		mediaUrlPattern:     "https://youtu.be/%s",
		mediaListUrlPattern: "https://www.youtube.com/playlist?list=%s",
		searchPrefix:        "ytsearch1:",
	}
}

var idRegex = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

func checkVideoId(id string) bool {
	return len(id) == 11 && idRegex.MatchString(id)
}

func checkPlaylistId(id string) bool {
	return idRegex.MatchString(id)
}

func videoURL(id string) *url.URL {
	u, err := url.Parse("https://youtu.be/" + id)
	if err != nil {
		panic("unexpected URL parse error")
	}

	return u
}

func playlistURL(id string) *url.URL {
	u, err := url.Parse("https://youtube.com/playlist?list=" + id)
	if err != nil {
		panic("unexpected URL parse error")
	}

	return u
}

func isoDurationToGoDuration(d duration.Duration) time.Duration {
	return time.Duration(d.Y)*time.Hour*24*365 +
		time.Duration(d.M)*time.Hour*24*30 +
		time.Duration(d.W)*time.Hour*24*7 +
		time.Duration(d.D)*time.Hour*24 +
		time.Duration(d.TH)*time.Hour +
		time.Duration(d.TM)*time.Minute +
		time.Duration(d.TS)*time.Second
}

const MediaKindYoutube MediaKind = "yt"

var ErrInvalidYTURL = errors.New("Invalid YouTube URL")

type YoutubeSource struct {
	resolver YoutubeResolver
}

type YoutubeVideoQuery struct {
	source *YoutubeSource
	query  string
}

func newYoutubeVideoQuery(src *YoutubeSource, query string) *YoutubeVideoQuery {
	return &YoutubeVideoQuery{source: src, query: query}
}

func (v *YoutubeVideoQuery) Kind() MediaKind {
	return MediaKindYoutube
}

func (v *YoutubeVideoQuery) Canonicalize(ctx context.Context) (CanonicalizedMediaObject, error) {
	return v.source.resolver.SearchMedia(v.source, ctx, v.query)
}

type YoutubeResolver interface {
	SearchMedia(src IdMediaSource[string], ctx context.Context, query string) (CanonicalizedMediaObject, error)
	ResolveMedia(src IdMediaSource[string], ctx context.Context, id string) (ResolvedMediaObjectSingle, error)
	ResolveMediaList(src IdMediaSource[string], ctx context.Context, id string) (ResolvedMediaObject, error)
}

type YoutubeAPI struct {
	apiKey string
}

func (yt *YoutubeAPI) newClient(ctx context.Context) (*youtube.Service, error) {
	return youtube.NewService(ctx, option.WithAPIKey(yt.apiKey))
}

func (yt *YoutubeAPI) SearchMedia(src IdMediaSource[string], ctx context.Context, query string) (CanonicalizedMediaObject, error) {
	client, err := yt.newClient(ctx)
	if err != nil {
		return nil, err
	}

	response, err := client.Search.List([]string{"part", "snippet", "contentDetails"}).Q(query).MaxResults(1).Type("video").Do()
	if err != nil {
		return nil, err
	}

	if len(response.Items) < 1 {
		return nil, ErrMediaNotFound
	}

	video := response.Items[1]
	id := video.Id.VideoId
	return NewIdMediaObject[string](src, id, nil), nil
}

func (yt *YoutubeAPI) ResolveMedia(src IdMediaSource[string], ctx context.Context, id string) (ResolvedMediaObjectSingle, error) {
	client, err := yt.newClient(ctx)
	if err != nil {
		return nil, err
	}

	response, err := client.Videos.List([]string{"snippet", "contentDetails"}).Id(id).MaxResults(1).Do()
	if err != nil {
		return nil, err
	}

	if len(response.Items) < 1 {
		return nil, ErrMediaNotFound
	}

	video := response.Items[0]
	videoLength, err := duration.ParseISO8601(video.ContentDetails.Duration)
	if err != nil && video.ContentDetails.Duration != "" {
		return nil, err
	}

	return NewIdMediaObject[string](src, id, &IdMediaObjectResolveInfo{
		id:          id,
		title:       video.Snippet.Title,
		artist:      video.Snippet.ChannelTitle,
		length:      isoDurationToGoDuration(videoLength),
		aspectRatio: "16/9",
	}), nil
}

func (yt *YoutubeAPI) ResolveMediaList(src IdMediaSource[string], ctx context.Context, id string) (ResolvedMediaObject, error) {
	// unsupported due to quota and shit
	return NewYoutubeDL().ResolveMediaList(ctx, id)
}

func processYoutubeURL(src *YoutubeSource, u *url.URL) (MediaObject, error) {
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, ErrUnsupportedURL
	}

	path := u.Path
	query := u.Query()

	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	if u.Hostname() != "youtu.be" && u.Hostname() != "yt.be" && u.Hostname() != "youtube.com" && u.Hostname() != "www.youtube.com" {
		return nil, ErrUnsupportedURL
	}

	if path == "watch" {
		id := query.Get("v")
		if !checkVideoId(id) {
			return nil, ErrInvalidYTURL
		}

		return NewIdMediaObject(src, id, nil), nil
	}

	if path == "playlist" {
		id := query.Get("list")
		if !checkPlaylistId(id) {
			return nil, ErrInvalidYTURL
		}

		return NewIdMediaListObject(src, id, nil), nil
	}

	if checkVideoId(path) {
		return NewIdMediaObject(src, path, nil), nil
	}

	if path != "" {
		return nil, ErrInvalidYTURL
	}

	if query.Has("search_query") {
		return newYoutubeVideoQuery(src, query.Get("search_query")), nil
	}

	if query.Has("search") {
		return newYoutubeVideoQuery(src, query.Get("search")), nil
	}

	return nil, ErrInvalidYTURL
}

func NewYoutubeAPI(apiKey string) *YoutubeSource {
	return &YoutubeSource{resolver: &YoutubeAPI{apiKey: apiKey}}
}

func NewYoutubeDL() *YoutubeSource {
	return &YoutubeSource{resolver: NewYoutubeYtdlResolver()}
}

func (yt *YoutubeSource) ProcessURL(u *url.URL) (MediaObject, error) {
	return processYoutubeURL(yt, u)
}

func (yt *YoutubeSource) Kind() MediaKind {
	return MediaKindYoutube
}

func (yt *YoutubeSource) MediaURL(id string) *url.URL {
	return NewYoutubeYtdlResolver().MediaURL(id)
}

func (yt *YoutubeSource) MediaListURL(id string) *url.URL {
	return NewYoutubeYtdlResolver().MediaListURL(id)
}

func (yt *YoutubeSource) ResolveMedia(ctx context.Context, id string) (ResolvedMediaObjectSingle, error) {
	return yt.resolver.ResolveMedia(yt, ctx, id)
}

func (yt *YoutubeSource) ResolveMediaList(ctx context.Context, id string) (ResolvedMediaObject, error) {
	return yt.resolver.ResolveMediaList(yt, ctx, id)
}
