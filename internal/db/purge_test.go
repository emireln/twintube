package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPurgeExpiredRoomsRemovesOnlyExpiredSavedRooms(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_TYPE", "sqlite")
	previous := Database
	database, err := InitDB(filepath.Join(t.TempDir(), "purge.sqlite"))
	if err != nil {
		t.Fatalf("initialize test database: %v", err)
	}
	t.Cleanup(func() {
		Database = previous
		_ = database.db.Close()
	})

	// Expiry is set in a non-UTC zone. The comparison must still hold: an hour in the past
	// is expired and two hours ahead is not, whatever offset the timestamps were written with.
	offset := time.FixedZone("UTC-3", -3*3600)
	past := time.Now().In(offset).Add(-time.Hour)
	future := time.Now().In(offset).Add(2 * time.Hour)
	if err := database.CreateRoomRecord("stale001", "Stale", "owner-1", "", false, &past, ""); err != nil {
		t.Fatalf("create stale room: %v", err)
	}
	if err := database.CreateRoomRecord("fresh001", "Fresh", "owner-1", "", false, &future, ""); err != nil {
		t.Fatalf("create fresh room: %v", err)
	}
	for _, roomID := range []string{"stale001", "fresh001"} {
		if _, err := database.db.Exec(`INSERT INTO messages (room_id, nickname, content) VALUES (?, ?, ?);`, roomID, "Guest", "hello"); err != nil {
			t.Fatalf("create message: %v", err)
		}
	}

	if err := database.PurgeExpiredRooms(time.Now()); err != nil {
		t.Fatalf("purge: %v", err)
	}

	count := func(query, id string) int {
		t.Helper()
		var n int
		if err := database.db.QueryRow(query, id).Scan(&n); err != nil {
			t.Fatalf("count %q: %v", query, err)
		}
		return n
	}
	if n := count(`SELECT COUNT(*) FROM rooms WHERE id = ?;`, "stale001"); n != 0 {
		t.Fatalf("expired room should be purged, found %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM messages WHERE room_id = ?;`, "stale001"); n != 0 {
		t.Fatalf("expired room's messages should be purged, found %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM rooms WHERE id = ?;`, "fresh001"); n != 1 {
		t.Fatalf("room that has not expired must remain, found %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM messages WHERE room_id = ?;`, "fresh001"); n != 1 {
		t.Fatalf("messages of a live room must remain, found %d", n)
	}
}
