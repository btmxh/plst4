package media

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/wader/goutubedl"
)

type YtdlResolver struct {
	mediaUrlPattern     string
	mediaListUrlPattern string
	searchPrefix        string
}

func NewYtdlResolver(mediaUrlPattern, mediaListUrlPattern, searchPrefix string) *YtdlResolver {
	return &YtdlResolver{mediaUrlPattern: mediaUrlPattern, mediaListUrlPattern: mediaListUrlPattern, searchPrefix: searchPrefix}
}

func (yt *YtdlResolver) MediaURL(id string) *url.URL {
	u, err := url.Parse(fmt.Sprintf(yt.mediaUrlPattern, id))
	if err != nil {
		panic("unexpected URL parse error")
	}

	return u
}

func (yt *YtdlResolver) MediaListURL(id string) *url.URL {
	if yt.mediaListUrlPattern == "" {
		panic("Media list not supported for this platform")
	}

	u, err := url.Parse(fmt.Sprintf(yt.mediaListUrlPattern, id))
	if err != nil {
		panic("unexpected URL parse error")
	}

	return u
}

func (yt *YtdlResolver) SearchMedia(src IdMediaSource[string], ctx context.Context, query string) (CanonicalizedMediaObject, error) {
	panic("unsupported")
}

func (yt *YtdlResolver) ResolveMedia(src IdMediaSource[string], ctx context.Context, id string) (ResolvedMediaObjectSingle, error) {
	result, err := goutubedl.New(ctx, yt.MediaURL(id).String(), goutubedl.Options{
		Type: goutubedl.TypeSingle,
	})
	if err != nil {
		return nil, err
	}

	return NewIdMediaObject(src, id, &IdMediaObjectResolveInfo{
		id:          id,
		title:       firstNonEmpty(result.Info.Title, UnknownTitle),
		artist:      firstNonEmpty(result.Info.Channel, result.Info.Uploader, UnknownArtist),
		length:      time.Duration(result.Info.Duration) * time.Second,
		aspectRatio: fmt.Sprintf("%d/%d", int(result.Info.Width), int(result.Info.Height)),
	}), nil
}

func (yt *YtdlResolver) ResolveMediaList(src IdMediaSource[string], ctx context.Context, id string) (ResolvedMediaObject, error) {
	result, err := goutubedl.New(ctx, yt.MediaListURL(id).String(), goutubedl.Options{
		Type:         goutubedl.TypePlaylist,
		FlatPlaylist: true,
	})
	if err != nil {
		return nil, err
	}

	var medias []IdMediaObject[string]
	for _, video := range result.Info.Entries {
		medias = append(medias, *NewIdMediaObject(src, id, &IdMediaObjectResolveInfo{
			id:          id,
			title:       firstNonEmpty(video.Title, UnknownTitle),
			artist:      firstNonEmpty(video.Channel, video.Uploader),
			length:      time.Duration(video.Duration) * time.Second,
			aspectRatio: "16/9",
		}))
	}

	return NewIdMediaListObject(src, id, &IdMediaListObjectResolveInfo[string]{
		title:  result.Info.Title,
		artist: result.Info.Channel,
		medias: medias,
	}), nil
}
