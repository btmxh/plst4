package media

import (
	"context"
	"net/url"
	"time"
)

type IdMediaSource[ID any] interface {
	Kind() MediaKind
	MediaURL(id ID) *url.URL
	MediaListURL(id ID) *url.URL
	ResolveMedia(ctx context.Context, id ID) (ResolvedMediaObjectSingle, error)
	ResolveMediaList(ctx context.Context, id ID) (ResolvedMediaObject, error)
}

type IdMediaObjectResolveInfo struct {
	id          string
	title       string
	artist      string
	length      time.Duration
	aspectRatio string
}

type IdMediaListObjectResolveInfo[ID any] struct {
	title  string
	artist string
	medias []IdMediaObject[ID]
}

type IdMediaObject[ID any] struct {
	source      IdMediaSource[ID]
	id          ID
	resolveInfo *IdMediaObjectResolveInfo
}

type IdMediaListObject[ID any] struct {
	source      IdMediaSource[ID]
	id          ID
	resolveInfo *IdMediaListObjectResolveInfo[ID]
}

func (m *IdMediaObject[ID]) Kind() MediaKind {
	return m.source.Kind()
}

func (m *IdMediaObject[ID]) Canonicalize(ctx context.Context) (CanonicalizedMediaObject, error) {
	return m, nil
}

func (m *IdMediaObject[ID]) URL() *url.URL {
	return m.source.MediaURL(m.id)
}

func (m *IdMediaObject[ID]) Resolve(ctx context.Context) (ResolvedMediaObject, error) {
	return m.source.ResolveMedia(ctx, m.id)
}

func (m *IdMediaObject[ID]) Title() string {
	return m.resolveInfo.title
}

func (m *IdMediaObject[ID]) Artist() string {
	return m.resolveInfo.artist
}

func (m *IdMediaObject[ID]) Duration() time.Duration {
	return m.resolveInfo.length
}

func (m *IdMediaObject[ID]) AspectRatio() string {
	return m.resolveInfo.aspectRatio
}

func (m *IdMediaObject[ID]) ChildEntries() []ResolvedMediaObjectSingle {
	return nil
}

func (m *IdMediaListObject[ID]) Kind() MediaKind {
	return m.source.Kind()
}

func (m *IdMediaListObject[ID]) Canonicalize(ctx context.Context) (CanonicalizedMediaObject, error) {
	return m, nil
}

func (m *IdMediaListObject[ID]) URL() *url.URL {
	return m.source.MediaListURL(m.id)
}

func (m *IdMediaListObject[ID]) Resolve(ctx context.Context) (ResolvedMediaObject, error) {
	return m.source.ResolveMediaList(ctx, m.id)
}

func (m *IdMediaListObject[ID]) Title() string {
	return m.resolveInfo.title
}

func (m *IdMediaListObject[ID]) Artist() string {
	return m.resolveInfo.artist
}

func (m *IdMediaListObject[ID]) ChildEntries() (medias []ResolvedMediaObjectSingle) {
	for _, media := range m.resolveInfo.medias {
		medias = append(medias, &media)
	}
	return
}

func NewIdMediaObject[ID any](source IdMediaSource[ID], id ID, resolveInfo *IdMediaObjectResolveInfo) *IdMediaObject[ID] {
	return &IdMediaObject[ID]{
		source:      source,
		id:          id,
		resolveInfo: resolveInfo,
	}
}

func NewIdMediaListObject[ID any](source IdMediaSource[ID], id ID, resolveInfo *IdMediaListObjectResolveInfo[ID]) *IdMediaListObject[ID] {
	return &IdMediaListObject[ID]{
		source:      source,
		id:          id,
		resolveInfo: resolveInfo,
	}
}
