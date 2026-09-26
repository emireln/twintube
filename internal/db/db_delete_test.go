package db

import (
	"path/filepath"
	"testing"
)

func TestDeleteOwnedRoomChecksOwnerBeforeDeletingRoomData(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_TYPE", "sqlite")
	previous := Database
	database, err := InitDB(filepath.Join(t.TempDir(), "delete-room.sqlite"))
	if err != nil {
		t.Fatalf("initialize test database: %v", err)
	}
	t.Cleanup(func() {
		Database = previous
		_ = database.db.Close()
	})

	if err := database.CreateRoomRecord("owned01", "Owned room", "owner-1", "", false, nil, ""); err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := database.db.Exec(`INSERT INTO messages (room_id, nickname, content) VALUES (?, ?, ?);`, "owned01", "Guest", "keep me"); err != nil {
		t.Fatalf("create room message: %v", err)
	}

	if err := database.DeleteOwnedRoom("owned01", "attacker"); err == nil {
		t.Fatal("expected an ownership error")
	}

	var roomCount, messageCount int
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM rooms WHERE id = ?;`, "owned01").Scan(&roomCount); err != nil {
		t.Fatalf("count room: %v", err)
	}
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE room_id = ?;`, "owned01").Scan(&messageCount); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if roomCount != 1 || messageCount != 1 {
		t.Fatalf("unauthorized delete changed data: rooms=%d messages=%d", roomCount, messageCount)
	}

	if err := database.DeleteOwnedRoom("owned01", "owner-1"); err != nil {
		t.Fatalf("owner should be allowed to delete room: %v", err)
	}
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM rooms WHERE id = ?;`, "owned01").Scan(&roomCount); err != nil {
		t.Fatalf("count deleted room: %v", err)
	}
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE room_id = ?;`, "owned01").Scan(&messageCount); err != nil {
		t.Fatalf("count deleted messages: %v", err)
	}
	if roomCount != 0 || messageCount != 0 {
		t.Fatalf("owner delete left data: rooms=%d messages=%d", roomCount, messageCount)
	}
}
