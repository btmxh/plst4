package services

import (
	"database/sql"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/btmxh/plst4/internal/db"
)

type QueuePlaylistItem struct {
	Title    string
	Artist   string
	URL      string
	Duration time.Duration
	Id       int
	Index    int
}

type MoveDirection int

const (
	MoveUp   MoveDirection = 1
	MoveDown MoveDirection = -1
)

type MoveItem struct {
	id    int
	order int
}

func EnumeratePlaylistItems(tx *db.Tx, playlist int, pageNum int) (page Pagination[QueuePlaylistItem], hasError bool) {
	if pageNum == 0 {
		// last page
		var itemCount int
		if tx.QueryRow("SELECT COUNT(*) FROM playlist_items WHERE playlist = $1", playlist).Scan(nil, &itemCount) {
			return page, true
		}

		pageNum = (itemCount + DefaultPagingLimit - 1) / DefaultPagingLimit
		slog.Warn("page", "p", itemCount)
	}

	var items []QueuePlaylistItem
	var rows *sql.Rows
	offset := (pageNum - 1) * DefaultPagingLimit

	if tx.Query(&rows, `
    SELECT 
        i.id, 
        COALESCE(a.alt_title, m.title),
        COALESCE(a.alt_artist, m.artist),
        m.url, 
        m.duration 
    FROM playlist_items i 
    JOIN medias m ON m.id = i.media 
		LEFT JOIN alt_metadata a ON a.media = m.id AND a.playlist = i.playlist
    WHERE i.playlist = $1 
    ORDER BY i.item_order 
    OFFSET $2 LIMIT $3`, playlist, offset, DefaultPagingLimit+1) {
		return page, true
	}

	for index := offset; rows.Next(); index += 1 {
		var item QueuePlaylistItem
		var duration time.Duration
		err := rows.Scan(&item.Id, &item.Title, &item.Artist, &item.URL, &duration)
		if err != nil {
			tx.PrivateError(err)
			return page, true
		}
		item.Duration = time.Duration(duration) * time.Second
		item.Index = index
		items = append(items, item)
	}

	return NewPagination(offset, items), false
}

func LocalRebalance(tx *db.Tx, playlist int, startOrder int, endOrder int, beforeItem int, numInsertions int) (begin int, delta int, hasErr bool) {
	alpha := 1.5
	beta := 2

	var count int
	for true {
		if tx.QueryRow("SELECT COUNT(*) FROM playlist_items WHERE playlist = $1 AND (item_order BETWEEN $2 AND $3)", playlist, startOrder, endOrder).Scan(nil, &count) {
			return begin, delta, true
		}

		if endOrder-startOrder >= beta*(numInsertions+count) {
			break
		}

		length := endOrder - startOrder
		startOrder = int(float64(startOrder) - alpha*float64(length))
		endOrder = int(float64(endOrder) + alpha*float64(length))
	}

	var beforeItemOrder int
	if tx.QueryRow("SELECT item_order FROM playlist_items WHERE id = $1", beforeItem).Scan(nil, &beforeItemOrder) {
		return begin, delta, true
	}

	delta = (endOrder - startOrder) / (numInsertions + count - 1)
	if count > 2 {
		slog.Info("Local rebalancing with", "startOrder", startOrder, "endOrder", endOrder, "beforeItem", beforeItem, "beforeItemOrder", beforeItemOrder, "numInsertions", numInsertions, "count", count, "delta", delta)
		if tx.Exec(nil,
			`
		WITH RankedRows AS (
			SELECT
				item_order,
				ROW_NUMBER() OVER (ORDER BY item_order) AS i
			FROM playlist_items
			WHERE playlist = $1 AND item_order BETWEEN $2 AND $3
		), UpdatedRows AS (
			SELECT
				item_order,
				($2 + $4 * (i - 1) + (CASE WHEN (item_order > $5) THEN ($4 * $6) ELSE 0 END)) AS new_order
			FROM RankedRows
		)
		UPDATE playlist_items
		SET item_order = UpdatedRows.new_order
		FROM UpdatedRows
		WHERE playlist_items.item_order = UpdatedRows.item_order AND playlist = $1
		`, playlist, startOrder, endOrder, delta, beforeItemOrder, numInsertions) {
			return begin, delta, true
		}

		if tx.QueryRow("SELECT item_order FROM playlist_items WHERE id = $1", beforeItem).Scan(nil, &beforeItemOrder) {
			return begin, delta, true
		}
	} else {
		slog.Info("Skipping local rebalancing due to insufficient items", "count", count, "startOrder", startOrder, "endOrder", endOrder, "beforeItem", beforeItem, "beforeItemOrder", beforeItemOrder, "numInsertions", numInsertions, "delta", delta)
	}

	begin = beforeItemOrder + delta
	return begin, delta, false
}

func AddPlaylistItems(tx *db.Tx, playlist int, mediaIds []int, beginOrder, deltaOrder int) (ids []int, hasErr bool) {
	for i, mediaId := range mediaIds {
		var itemId int
		if tx.QueryRow("INSERT INTO playlist_items (playlist, media, item_order) VALUES ($1, $2, $3) RETURNING id", playlist, mediaId, beginOrder+i*int(deltaOrder)).Scan(nil, &itemId) {
			return ids, true
		}
		ids = append(ids, itemId)
	}

	return ids, false
}

func GetPlaylistItemOrder(tx *db.Tx, id int) (order int, hasErr bool) {
	var hasRow bool
	hasErr = tx.QueryRow("SELECT item_order FROM playlist_items WHERE id = $1", id).Scan(&hasRow, &order)
	if !hasRow {
		tx.PrivateError(sql.ErrNoRows)
		tx.PublicError(http.StatusNotFound, db.GenericError)
	}
	return order, hasErr || !hasRow
}

func GetNextPlaylistItem(tx *db.Tx, playlist, prevOrder int) (id int, order int, hasRow, hasErr bool) {
	hasErr = tx.QueryRow("SELECT id, item_order FROM playlist_items WHERE item_order > $1 AND playlist = $2 ORDER BY item_order ASC LIMIT 1", prevOrder, playlist).Scan(&hasRow, &id, &order)
	return id, order, hasRow, hasErr
}

func CheckPlaylistItemExists(tx *db.Tx, playlist, id int) (hasRow bool, hasErr bool) {
	var dummy int
	hasErr = tx.QueryRow("SELECT 1 FROM playlist_items WHERE id = $1 AND playlist = $2", id, playlist).Scan(&hasRow, &dummy)
	return hasRow, hasErr
}

func DeletePlaylistItem(tx *db.Tx, playlist int, id int) (hasErr bool) {
	return tx.Exec(nil, "DELETE FROM playlist_items WHERE playlist = $1 AND id = $2", playlist, id)
}

func MoveItems(tx *db.Tx, playlist int, items []int, dir MoveDirection) (numAffected int, hasErr bool) {
	var moveItems []MoveItem
	for _, itemId := range items {
		var item MoveItem
		item.id = itemId
		if tx.QueryRow("SELECT item_order FROM playlist_items WHERE playlist = $1 AND id = $2", playlist, itemId).Scan(nil, &item.order) {
			return 0, true
		}

		moveItems = append(moveItems, item)
	}

	sort.Slice(moveItems, func(i, j int) bool {
		return int(dir)*moveItems[i].order > int(dir)*moveItems[j].order
	})

	prevAfter := -1
	affected := make(map[int]struct{})

	for _, item := range moveItems {
		var afterId int
		var after int
		var hasRow bool
		sign := ">"
		order := "ASC"
		if dir == MoveDown {
			sign = "<"
			order = "DESC"
		}
		if tx.QueryRow("SELECT id, item_order FROM playlist_items WHERE playlist = $1 AND item_order "+sign+" (SELECT item_order FROM playlist_items WHERE id = $2) ORDER BY item_order "+order+" LIMIT 1", playlist, item.id).Scan(&hasRow, &afterId, &after) {
			return 0, true
		}
		if !hasRow || afterId == prevAfter {
			prevAfter = item.id
			continue
		}

		if tx.Exec(nil, `
		UPDATE playlist_items SET item_order = (CASE id
			WHEN $1 THEN (SELECT item_order FROM playlist_items WHERE id = $2)
			WHEN $2 THEN (SELECT item_order FROM playlist_items WHERE id = $1)
		END)::INTEGER WHERE id IN ($1, $2)
		`, item.id, afterId) {
			return
		}

		affected[item.id] = struct{}{}
		affected[afterId] = struct{}{}
	}

	return len(affected), false
}
