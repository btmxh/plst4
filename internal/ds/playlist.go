package ds

import (
	"github.com/google/uuid"
)

type Id = uuid.NullUUID
type NonNilId = uuid.UUID

func NilId() Id {
	var id Id
	return id
}

func Nilable(id NonNilId) Id {
	return Id{Valid: true, UUID: id}
}

type PlaylistItem struct {
	Id    Id
	Index int
}

type PlaylistInfo struct {
	First      PlaylistItem
	Last       PlaylistItem
	Current    PlaylistItem
	HasCurrent bool
	Count      int
}

type PlaylistItemInfo struct {
	Id   Id
	Prev Id
	Next Id
}

type Playlist interface {
	Query() (PlaylistInfo, error)
	QueryItem(id NonNilId) (PlaylistItemInfo, error)

	SetFirst(id Id) error
	SetLast(id Id) error
	SetCount(count int) error
	SetCurrent(id Id, index int) error

	SetPrev(id NonNilId, next Id) error
	SetNext(id NonNilId, next Id) error

	PhysicalDeleteItem(id NonNilId) error
}

type AddPosition string

const (
	AddToStart AddPosition = "add-to-start"
	AddToEnd   AddPosition = "add-to-end"
	QueueNext  AddPosition = "queue-next"
)

func insertBetween(p Playlist, pi PlaylistInfo, item NonNilId, prev Id, next Id, index int) error {
	// link prev and item
	if err := p.SetPrev(item, prev); err != nil {
		return err
	}
	if prev.Valid {
		if err := p.SetNext(prev.UUID, Nilable(item)); err != nil {
			return err
		}
	} else {
		if err := p.SetFirst(Nilable(item)); err != nil {
			return err
		}
	}

	// link item and next
	if err := p.SetNext(item, next); err != nil {
		return err
	}
	if next.Valid {
		if err := p.SetPrev(next.UUID, Nilable(item)); err != nil {
			return err
		}
	} else {
		if err := p.SetLast(Nilable(item)); err != nil {
			return err
		}
	}

	// update count
	if err := p.SetCount(pi.Count + 1); err != nil {
		return err
	}

	// update current index
	if pi.Current.Id == next && next != NilId() {
		if err := p.SetCurrent(next, index+1); err != nil {
			return err
		}
	}

	return nil
}

func AddItem(p Playlist, item NonNilId, pos AddPosition) error {
	info, err := p.Query()
	if err != nil {
		return err
	}

	if !info.HasCurrent && pos == QueueNext {
		pos = AddToEnd
	}

	switch pos {
	case AddToStart:
		return insertBetween(p, info, item, NilId(), info.First.Id, 0)
	case AddToEnd:
		return insertBetween(p, info, item, info.Last.Id, NilId(), info.Count)
	case QueueNext:
		currentInfo, err := p.QueryItem(info.Current.Id.UUID)
		if err != nil {
			return err
		}

		return insertBetween(p, info, item, currentInfo.Id, currentInfo.Next, info.Current.Index+1)
	}

	return nil
}

func DeleteItem(p Playlist, item PlaylistItem) error {
	if !item.Id.Valid {
		return nil
	}

	info, err := p.Query()
	if err != nil {
		return err
	}

	itemInfo, err := p.QueryItem(item.Id.UUID)
	if err != nil {
		return err
	}

	if itemInfo.Prev.Valid {
		if err := p.SetNext(itemInfo.Prev.UUID, itemInfo.Next); err != nil {
			return err
		}
	} else {
		if err := p.SetFirst(itemInfo.Next); err != nil {
			return err
		}
	}

	if itemInfo.Next.Valid {
		if err := p.SetPrev(itemInfo.Next.UUID, itemInfo.Prev); err != nil {
			return err
		}
	} else {
		if err := p.SetLast(itemInfo.Prev); err != nil {
			return err
		}
	}

	// update current
	if info.HasCurrent {
		if info.Current.Id == item.Id {
			if err := p.SetCurrent(NilId(), -1); err != nil {
				return err
			}
		} else {
			if info.Current.Index > item.Index {
				if err := p.SetCurrent(info.Current.Id, info.Current.Index-1); err != nil {
					return err
				}
			}
		}
	}

	return p.SetCount(info.Count - 1)
}
