package ds

import (
	"database/sql"

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
	First   PlaylistItem
	Last    PlaylistItem
	Current PlaylistItem
	Count   int
}

type PlaylistItemInfo struct {
	Id   NonNilId
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

	SetPrev(id NonNilId, prev Id) error
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

	if !info.Current.Id.Valid && pos == QueueNext {
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

		return insertBetween(p, info, item, Nilable(currentInfo.Id), currentInfo.Next, info.Current.Index+1)
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
	if info.Current.Id.Valid {
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

type PlaylistWrapper struct {
	id string
	tx *sql.Tx
}

func CreatePlaylistWrapper(id string, tx *sql.Tx) *PlaylistWrapper {
	return &PlaylistWrapper{id: id, tx: tx}
}

func (p *PlaylistWrapper) Query() (PlaylistInfo, error) {
	var info PlaylistInfo
	var id uuid.NullUUID
	err := p.tx.QueryRow("SELECT first, last, current, current_idx, item_count FROM playlists WHERE id = $1", p.id).Scan(
		&info.First.Id, &info.Last.Id, &id, &info.Current.Index, &info.Count,
	)

	info.First.Index = 0
	info.Last.Index = info.Count - 1

	return info, err
}

func (p *PlaylistWrapper) QueryItem(id NonNilId) (PlaylistItemInfo, error) {
	var info PlaylistItemInfo
	info.Id = id
	err := p.tx.QueryRow("SELECT prev, next FROM playlist_items WHERE id = $1 AND playlist = $2", id, p.id).Scan(&info.Prev, &info.Next)
	return info, err
}

func combineResultErr(res sql.Result, err error) error {
	if err != nil {
		return err
	}

	if affected, err := res.RowsAffected(); affected == 0 && err == nil {
		return sql.ErrNoRows
	}

	return nil
}

func (p *PlaylistWrapper) SetFirst(id Id) error {
	res, err := p.tx.Exec("UPDATE playlists SET first = $1 WHERE id = $2", id, p.id)
	return combineResultErr(res, err)
}

func (p *PlaylistWrapper) SetLast(id Id) error {
	res, err := p.tx.Exec("UPDATE playlists SET last = $1 WHERE id = $2", id, p.id)
	return combineResultErr(res, err)
}

func (p *PlaylistWrapper) SetCount(count int) error {
	res, err := p.tx.Exec("UPDATE playlists SET item_count = $1 WHERE id = $2", count, p.id)
	return combineResultErr(res, err)
}

func (p *PlaylistWrapper) SetCurrent(id Id, index int) error {
	res, err := p.tx.Exec("UPDATE playlists SET current = $1, current_idx = $2 WHERE id = $3", id, index, p.id)
	return combineResultErr(res, err)
}

func (p *PlaylistWrapper) SetPrev(id NonNilId, prev Id) error {
	res, err := p.tx.Exec("UPDATE playlist_items SET prev = $1 WHERE id = $2 AND playlist = $3", prev, id, p.id)
	return combineResultErr(res, err)
}

func (p *PlaylistWrapper) SetNext(id NonNilId, next Id) error {
	res, err := p.tx.Exec("UPDATE playlist_items SET next = $1 WHERE id = $2 AND playlist = $3", next, id, p.id)
	return combineResultErr(res, err)
}

func (p *PlaylistWrapper) PhysicalDeleteItem(id NonNilId) error {
	res, err := p.tx.Exec("DELETE FROM playlist_items WHERE id = $1 AND playlist = $2", id, p.id)
	return combineResultErr(res, err)
}
