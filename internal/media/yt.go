package media

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/senseyeio/duration"
	"github.com/wader/goutubedl"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func checkId(s string) bool {
	for _, r := range s {
		suitable := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !suitable {
			return false
		}
	}

	return true
}

func checkVideoId(id string) bool {
	return len(id) == 11 && checkId(id)
}

func checkPlaylistId(id string) bool {
	return checkId(id)
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

type YoutubeVideoResolveInfo struct {
	id          string
	title       string
	artist      string
	length      time.Duration
	aspectRatio string
}

func (v *YoutubeVideoResolveInfo) Kind() MediaKind {
	return MediaKindYoutube
}

func (v *YoutubeVideoResolveInfo) Canonicalize(_ context.Context) (CanonicalizedMediaObject, error) {
	return v, nil
}

func (v *YoutubeVideoResolveInfo) URL() *url.URL {
	return videoURL(v.id)
}

func (v *YoutubeVideoResolveInfo) Resolve(_ context.Context) (ResolvedMediaObject, error) {
	return v, nil
}

func (v *YoutubeVideoResolveInfo) Title() string {
	return v.title
}

func (v *YoutubeVideoResolveInfo) Artist() string {
	return v.artist
}

func (v *YoutubeVideoResolveInfo) ChildEntries() []ResolvedMediaObjectSingle {
	return nil
}

func (v *YoutubeVideoResolveInfo) Duration() time.Duration {
	return v.length
}

func (v *YoutubeVideoResolveInfo) AspectRatio() string {
	return v.aspectRatio
}

type YoutubePlaylistResolveInfo struct {
	title  string
	artist string
	videos []YoutubeVideoResolveInfo
}

type YoutubeVideo struct {
	resolver    YoutubeResolver
	id          string
	resolveInfo *YoutubeVideoResolveInfo
}

func newYoutubeVideo(resolver YoutubeResolver, id string, resolveInfo *YoutubeVideoResolveInfo) *YoutubeVideo {
	return &YoutubeVideo{resolver: resolver, id: id, resolveInfo: resolveInfo}
}

func (v *YoutubeVideo) Kind() MediaKind {
	return MediaKindYoutube
}

func (v *YoutubeVideo) Canonicalize(ctx context.Context) (CanonicalizedMediaObject, error) {
	return v, nil
}

func (v *YoutubeVideo) URL() *url.URL {
	return videoURL(v.id)
}

func (v *YoutubeVideo) Resolve(ctx context.Context) (ResolvedMediaObject, error) {
	if v.resolveInfo == nil {
		return v.resolver.ResolveMedia(ctx, v.id)
	} else {
		return v, nil
	}
}

func (v *YoutubeVideo) Title() string {
	return v.resolveInfo.Title()
}

func (v *YoutubeVideo) Artist() string {
	return v.resolveInfo.Artist()
}

func (v *YoutubeVideo) Duration() time.Duration {
	return v.resolveInfo.Duration()
}

func (v *YoutubeVideo) AspectRatio() string {
	return v.resolveInfo.AspectRatio()
}

func (v *YoutubeVideo) ChildEntries() []ResolvedMediaObjectSingle {
	return v.resolveInfo.ChildEntries()
}

type YoutubeVideoQuery struct {
	resolver YoutubeResolver
	query    string
}

func newYoutubeVideoQuery(resolver YoutubeResolver, query string) *YoutubeVideoQuery {
	return &YoutubeVideoQuery{resolver: resolver, query: query}
}

func (v *YoutubeVideoQuery) Kind() MediaKind {
	return MediaKindYoutube
}

func (v *YoutubeVideoQuery) Canonicalize(ctx context.Context) (CanonicalizedMediaObject, error) {
	return v.resolver.SearchMedia(ctx, v.query)
}

type YoutubePlaylist struct {
	resolver    YoutubeResolver
	id          string
	resolveInfo *YoutubePlaylistResolveInfo
}

func newYoutubePlaylist(resolver YoutubeResolver, id string, resolveInfo *YoutubePlaylistResolveInfo) *YoutubePlaylist {
	return &YoutubePlaylist{resolver: resolver, id: id, resolveInfo: resolveInfo}
}

func (v *YoutubePlaylist) Kind() MediaKind {
	return MediaKindYoutube
}

func (v *YoutubePlaylist) Canonicalize(ctx context.Context) (CanonicalizedMediaObject, error) {
	return v, nil
}

func (v *YoutubePlaylist) URL() *url.URL {
	return playlistURL(v.id)
}

func (v *YoutubePlaylist) Resolve(ctx context.Context) (ResolvedMediaObject, error) {
	if v.resolveInfo == nil {
		return v.resolver.ResolveMediaList(ctx, v.id)
	} else {
		return v, nil
	}
}

func (v *YoutubePlaylist) Title() string {
	return v.resolveInfo.title
}

func (v *YoutubePlaylist) Artist() string {
	return v.resolveInfo.artist
}

func (v *YoutubePlaylist) ChildEntries() (videos []ResolvedMediaObjectSingle) {
	for _, video := range v.resolveInfo.videos {
		videos = append(videos, &video)
	}
	return videos
}

// two resolvers implementation
type YoutubeResolver interface {
	SearchMedia(ctx context.Context, query string) (CanonicalizedMediaObject, error)
	ResolveMedia(ctx context.Context, id string) (ResolvedMediaObject, error)
	ResolveMediaList(ctx context.Context, id string) (ResolvedMediaObject, error)
}

type YoutubeAPI struct {
	apiKey string
}

func NewYoutubeAPI(apiKey string) *YoutubeAPI {
	return &YoutubeAPI{apiKey: apiKey}
}

func (yt *YoutubeAPI) newClient(ctx context.Context) (*youtube.Service, error) {
	return youtube.NewService(ctx, option.WithAPIKey(yt.apiKey))
}

func (yt *YoutubeAPI) SearchMedia(ctx context.Context, query string) (CanonicalizedMediaObject, error) {
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
	return newYoutubeVideo(yt, id, nil), nil
}

func (yt *YoutubeAPI) ResolveMedia(ctx context.Context, id string) (ResolvedMediaObject, error) {
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
	if err != nil {
		return nil, err
	}

	return newYoutubeVideo(yt, id, &YoutubeVideoResolveInfo{
		id:          id,
		title:       video.Snippet.Title,
		artist:      video.Snippet.ChannelTitle,
		length:      isoDurationToGoDuration(videoLength),
		aspectRatio: "16/9",
	}), nil
}

func (yt *YoutubeAPI) ResolveMediaList(ctx context.Context, id string) (ResolvedMediaObject, error) {
	// unsupported due to quota and shit
	return NewYoutubeDL().ResolveMediaList(ctx, id)
}

type YoutubeDL struct {
}

func NewYoutubeDL() *YoutubeDL {
	return &YoutubeDL{}
}

func fetchYTDL(ctx context.Context, url string) (goutubedl.Result, error) {
	return goutubedl.New(ctx, url, goutubedl.Options{})
}

func (yt *YoutubeDL) SearchMedia(ctx context.Context, query string) (CanonicalizedMediaObject, error) {
	panic("unsupported")
}

func (yt *YoutubeDL) ResolveMedia(ctx context.Context, id string) (ResolvedMediaObject, error) {
	result, err := fetchYTDL(ctx, videoURL(id).String())
	if err != nil {
		return nil, err
	}

	return newYoutubeVideo(yt, id, &YoutubeVideoResolveInfo{
		id:          id,
		title:       result.Info.Title,
		artist:      result.Info.Channel,
		length:      time.Duration(result.Info.Duration) * time.Second,
		aspectRatio: fmt.Sprintf("%d/%d", int(result.Info.Width), int(result.Info.Height)),
	}), nil
}

func (yt *YoutubeDL) ResolveMediaList(ctx context.Context, id string) (ResolvedMediaObject, error) {
	result, err := fetchYTDL(ctx, playlistURL(id).String())
	if err != nil {
		return nil, err
	}

	var videos []YoutubeVideoResolveInfo
	for _, video := range result.Info.Entries {
		videos = append(videos, YoutubeVideoResolveInfo{
			id:          id,
			title:       video.Title,
			artist:      video.Channel,
			length:      time.Duration(video.Duration) * time.Second,
			aspectRatio: fmt.Sprintf("%d/%d", int(video.Width), int(video.Height)),
		})
	}

	return newYoutubePlaylist(yt, id, &YoutubePlaylistResolveInfo{
		title:  result.Info.Title,
		artist: result.Info.Channel,
		videos: videos,
	}), nil
}

func processURL(resolver YoutubeResolver, u *url.URL) (MediaObject, error) {
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

		return newYoutubeVideo(resolver, id, nil), nil
	}

	if path == "playlist" {
		id := query.Get("list")
		if !checkPlaylistId(id) {
			return nil, ErrInvalidYTURL
		}

		return newYoutubePlaylist(resolver, id, nil), nil
	}

	if checkVideoId(path) {
		return newYoutubeVideo(resolver, path, nil), nil
	}

	if path != "" {
		return nil, ErrInvalidYTURL
	}

	if query.Has("search_query") {
		return newYoutubeVideoQuery(resolver, query.Get("search_query")), nil
	}

	if query.Has("search") {
		return newYoutubeVideoQuery(resolver, query.Get("search")), nil
	}

	return nil, ErrInvalidYTURL
}

func (yt *YoutubeDL) ProcessURL(u *url.URL) (MediaObject, error) {
	return processURL(yt, u)
}

func (yt *YoutubeAPI) ProcessURL(u *url.URL) (MediaObject, error) {
	return processURL(yt, u)
}
