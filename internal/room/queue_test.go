package room

import (
	"strconv"
	"testing"

	"twintube/internal/db"
)

func TestAppendPlaylistItemStopsAtLimit(t *testing.T) {
	r := &Room{}
	for i := 0; i < maxPlaylistItems; i++ {
		if _, _, added := r.AppendPlaylistItem(db.PlaylistItem{ID: strconv.Itoa(i)}); !added {
			t.Fatalf("item %d should fit under the limit", i+1)
		}
	}

	_, playlist, added := r.AppendPlaylistItem(db.PlaylistItem{ID: "one-too-many"})
	if added {
		t.Fatal("item beyond the limit must be refused")
	}
	if len(playlist) != maxPlaylistItems {
		t.Fatalf("queue should stay at %d items, got %d", maxPlaylistItems, len(playlist))
	}
	if r.Playlist[len(r.Playlist)-1].ID != strconv.Itoa(maxPlaylistItems-1) {
		t.Fatal("a refused item must not be appended")
	}
}
