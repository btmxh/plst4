package ds

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
)

type Item struct {
	id   NonNilId
	name string
	prev Id
	next Id
}

type TestPlaylist struct {
	Items        map[NonNilId]Item
	First        Id
	Last         Id
	Current      Id
	Count        int
	CurrentIndex int
}

func add(t *testing.T, p *TestPlaylist, name string, pos AddPosition) {
	uuid := uuid.New()
	p.Items[uuid] = Item{id: uuid, name: name}
	err := AddItem(p, uuid, pos)
	if err != nil {
		t.Fatalf("Unable to add to playlist: %v", err)
	}
}

func (p *TestPlaylist) node(id Id) string {
	if !id.Valid {
		return "nil"
	}

	return p.Items[id.UUID].name
}

func collect(t *testing.T, p *TestPlaylist) []Item {
	limit := 1000
	var slice []Item

	node := p.First
	for node.Valid {
		if len(slice) > limit {
			t.Fatal("Too many items in playlist")
		}

		item := p.Items[node.UUID]
		slice = append(slice, item)
		node = item.next
	}

	return slice
}

func check(t *testing.T, p *TestPlaylist, expected []string, current string) {
	values := collect(t, p)

	if p.Count != len(values) {
		t.Fatalf("Playlist count mismatch: %d != %d", p.Count, len(values))
	}

	if p.CurrentIndex >= p.Count {
		t.Fatalf("Playlist count (%d) <= currentIndex (%d)", p.Count, p.CurrentIndex)
	}

	// empty case
	if p.Count == 0 {
		if p.First != NilId() {
			t.Fatalf("Empty playlist with first (%s) != nil", p.node(p.First))
		}
		if p.Last != NilId() {
			t.Fatalf("Empty playlist with last (%s) != nil: ", p.node(p.Last))
		}
	} else {
		if p.First == NilId() {
			t.Fatalf("Non-empty playlist with first == nil")
		}
		if p.Last == NilId() {
			t.Fatalf("Non-empty playlist with last == nil")
		}

		indexToUUID := func(index int) Id {
			if index < 0 || index >= p.Count {
				return NilId()
			}

			return Nilable(values[index].id)
		}

		for index, value := range values {
			prev := indexToUUID(index - 1)
			next := indexToUUID(index + 1)
			if value.prev != prev {
				t.Fatalf("Prev pointer mismatch at element index %d: %s != %s", index, p.node(value.prev), p.node(prev))
			}
			if value.next != next {
				t.Fatalf("Next pointer mismatch at element index %d: %s != %s", index, p.node(value.next), p.node(next))
			}
		}

		if p.CurrentIndex >= 0 {
			if Nilable(values[p.CurrentIndex].id) != p.Current {
				t.Fatalf("Current mismatch: %s != %s", values[p.CurrentIndex].name, p.node(p.Current))
			}
		}

		if len(current) == 0 {
			current = "nil"
		}
		if p.node(p.Current) != current {
			t.Fatalf("Mismatch current item: %s != %s", p.node(p.Current), current)
		}
	}

	if p.Count != len(expected) {
		t.Fatalf("List count (%d) != expected count (%d)", p.Count, len(expected))
	}

	for index, value := range values {
		if value.name != expected[index] {
			t.Fatalf("Element mismatch at index %d: %s != %s", index, value.name, expected[index])
		}
	}
}

func pick(t *testing.T, p *TestPlaylist, index int, name string) {
	if index < 0 {
		p.SetCurrent(NilId(), -1)
	}

	items := collect(t, p)
	if name != items[index].name {
		t.Fatalf("Mismatch pick() name at index %d: %s != %s", index, name, items[index].name)
	}

	p.SetCurrent(Nilable(items[index].id), index)
}

func remove(t *testing.T, p *TestPlaylist, index int, name string) {
	items := collect(t, p)
	if name != items[index].name {
		t.Fatalf("Mismatch pick() name at index %d: %s != %s", index, name, items[index].name)
	}

	if err := DeleteItem(p, PlaylistItem{Id: Nilable(items[index].id), Index: index}); err != nil {
		t.Fatalf("Unable to delete item: %v", err)
	}
}

func (p *TestPlaylist) Query() (PlaylistInfo, error) {
	var info PlaylistInfo
	info.Count = p.Count
	info.First.Index = 0
	info.Last.Index = p.Count - 1
	info.Current.Index = p.CurrentIndex
	info.First.Id = p.First
	info.Last.Id = p.Last
	if info.Current.Index >= 0 {
		info.Current.Id = p.Current
	}
	return info, nil
}

func (p *TestPlaylist) QueryItem(id NonNilId) (PlaylistItemInfo, error) {
	item := p.Items[id]
	return PlaylistItemInfo{Id: id, Prev: item.prev, Next: item.next}, nil
}

func (p *TestPlaylist) SetFirst(id Id) error {
	p.First = id
	return nil
}

func (p *TestPlaylist) SetLast(id Id) error {
	p.Last = id
	return nil
}

func (p *TestPlaylist) SetCount(count int) error {
	p.Count = count
	return nil
}

func (p *TestPlaylist) SetCurrent(id Id, index int) error {
	p.CurrentIndex = index
	p.Current = id
	return nil
}

func (p *TestPlaylist) SetPrev(id NonNilId, prev Id) error {
	value := p.Items[id]
	value.prev = prev
	p.Items[id] = value
	return nil
}

func (p *TestPlaylist) SetNext(id NonNilId, next Id) error {
	value := p.Items[id]
	value.next = next
	p.Items[id] = value
	return nil
}

func (p *TestPlaylist) PhysicalDeleteItem(id NonNilId) error {
	delete(p.Items, id)
	return nil
}

func newPlaylist() *TestPlaylist {
	return &TestPlaylist{
		CurrentIndex: -1,
		Items:        make(map[uuid.UUID]Item),
	}
}

func TestEmpty(t *testing.T) {
	list := newPlaylist()
	check(t, list, []string{}, "")
}

func TestOneItemAddToStart(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToStart)
	check(t, list, []string{"foo"}, "")
}

func TestOneItemAddToEnd(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	check(t, list, []string{"foo"}, "")
}

func TestOneItemQueueNext(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", QueueNext)
	check(t, list, []string{"foo"}, "")
}

func TestTwoItemQueueNext(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", QueueNext)
	pick(t, list, 0, "foo")
	add(t, list, "bar", QueueNext)
	check(t, list, []string{"foo", "bar"}, "foo")
}

// tests by chatgpt
func TestThreeItemQueueNext(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", QueueNext)
	add(t, list, "baz", QueueNext)
	pick(t, list, 0, "foo")
	add(t, list, "bar", QueueNext)
	check(t, list, []string{"foo", "bar", "baz"}, "foo")
}
func TestMultipleItemsAddToStart(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToStart)
	add(t, list, "bar", AddToStart)
	add(t, list, "baz", AddToStart)
	check(t, list, []string{"baz", "bar", "foo"}, "")
}

func TestMultipleItemsAddToEnd(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "bar", AddToEnd)
	add(t, list, "baz", AddToEnd)
	check(t, list, []string{"foo", "bar", "baz"}, "")
}

func TestMultipleQueueNext(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", QueueNext)
	add(t, list, "baz", QueueNext)
	add(t, list, "bar", QueueNext)
	check(t, list, []string{"foo", "baz", "bar"}, "")
}

func TestRemoveItem(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "bar", AddToEnd)
	add(t, list, "baz", AddToEnd)
	remove(t, list, 1, "bar") // Remove the second item
	check(t, list, []string{"foo", "baz"}, "")
}

func TestPickDifferentItem(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "bar", AddToEnd)
	add(t, list, "baz", AddToEnd)
	pick(t, list, 1, "bar")
	add(t, list, "qux", QueueNext)
	check(t, list, []string{"foo", "bar", "qux", "baz"}, "bar")
}

func TestRemoveCurrentItem(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "bar", AddToEnd)
	add(t, list, "baz", AddToEnd)
	pick(t, list, 1, "bar")
	remove(t, list, 1, "bar") // Remove the current item
	check(t, list, []string{"foo", "baz"}, "")
}

func TestEmptyAfterRemoveAll(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "bar", AddToEnd)
	remove(t, list, 0, "foo")
	remove(t, list, 0, "bar")
	check(t, list, []string{}, "")
}

func TestCircularQueue(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "bar", AddToEnd)
	add(t, list, "baz", AddToEnd)
	pick(t, list, 2, "baz") // Pick the last item
	add(t, list, "qux", QueueNext)
	check(t, list, []string{"foo", "bar", "baz", "qux"}, "baz")
}

func TestAddAfterClear(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "bar", AddToEnd)
	remove(t, list, 0, "foo")
	remove(t, list, 0, "bar")
	check(t, list, []string{}, "")
	add(t, list, "baz", AddToEnd)
	check(t, list, []string{"baz"}, "")
}

func TestLargePlaylist(t *testing.T) {
	list := newPlaylist()
	for i := 1; i <= 100; i++ {
		add(t, list, fmt.Sprintf("item%d", i), AddToEnd)
	}
	expected := make([]string, 100)
	for i := 1; i <= 100; i++ {
		expected[i-1] = fmt.Sprintf("item%d", i)
	}
	check(t, list, expected, "")
}

func TestQueueNextWithNoActiveItem(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", QueueNext) // Falls back to AddToEnd
	add(t, list, "bar", QueueNext) // Falls back to AddToEnd
	check(t, list, []string{"foo", "bar"}, "")
}

func TestQueueNextWithActiveItem(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "baz", AddToEnd)
	pick(t, list, 0, "foo")        // "foo" becomes active
	add(t, list, "bar", QueueNext) // Insert after "foo"
	check(t, list, []string{"foo", "bar", "baz"}, "foo")
}

func TestQueueNextAfterRemoveActiveItem(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "baz", AddToEnd)
	pick(t, list, 0, "foo")        // "foo" becomes active
	remove(t, list, 0, "foo")      // Remove the active item
	add(t, list, "bar", QueueNext) // Falls back to AddToEnd
	check(t, list, []string{"baz", "bar"}, "")
}

func TestAddToStartAfterQueueNext(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", QueueNext) // Falls back to AddToEnd
	add(t, list, "bar", AddToStart)
	check(t, list, []string{"bar", "foo"}, "")
}

func TestQueueNextWithActiveLastItem(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "bar", AddToEnd)
	add(t, list, "baz", AddToEnd)
	pick(t, list, 2, "baz")        // "baz" becomes active
	add(t, list, "qux", QueueNext) // Insert after "baz"
	check(t, list, []string{"foo", "bar", "baz", "qux"}, "baz")
}

func TestRemoveAndAddWithQueueNext(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	add(t, list, "bar", AddToEnd)
	add(t, list, "baz", AddToEnd)
	pick(t, list, 1, "bar")        // "bar" becomes active
	remove(t, list, 1, "bar")      // Remove "bar"
	add(t, list, "qux", QueueNext) // Falls back to AddToEnd
	check(t, list, []string{"foo", "baz", "qux"}, "")
}

func TestEmptyPlaylistWithQueueNext(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", QueueNext) // Falls back to AddToEnd
	remove(t, list, 0, "foo")      // Remove the only item
	add(t, list, "bar", QueueNext) // Falls back to AddToEnd
	check(t, list, []string{"bar"}, "")
}

func TestMultipleQueueNextInterleavedWithAddToEnd(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", QueueNext) // Falls back to AddToEnd
	add(t, list, "bar", QueueNext) // Falls back to AddToEnd
	add(t, list, "baz", AddToEnd)
	add(t, list, "qux", QueueNext) // Falls back to AddToEnd
	check(t, list, []string{"foo", "bar", "baz", "qux"}, "")
}

func TestAddRemoveAddQueueNext(t *testing.T) {
	list := newPlaylist()
	add(t, list, "foo", AddToEnd)
	remove(t, list, 0, "foo")
	add(t, list, "bar", AddToEnd)
	add(t, list, "baz", QueueNext) // Falls back to AddToEnd
	check(t, list, []string{"bar", "baz"}, "")
}
