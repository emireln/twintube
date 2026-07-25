package room

import (
	"sync"
	"testing"
)

func newTestManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room),
		mu:    sync.RWMutex{},
	}
}

func TestLookupJoinMetaUnknownRoom(t *testing.T) {
	rm := newTestManager()
	meta := rm.LookupJoinMeta("zzzzzzzz")
	if meta.Found {
		t.Fatalf("expected unknown room to be not found")
	}
}

func TestOpenExistingRoomDoesNotCreatePhantom(t *testing.T) {
	rm := newTestManager()
	got := rm.OpenExistingRoom("nofile01")
	if got != nil {
		t.Fatalf("expected nil for unknown room, got %+v", got)
	}
	if rm.GetRoom("nofile01") != nil {
		t.Fatalf("phantom room was created in memory")
	}
}

func TestOpenExistingRoomReturnsGuestRoom(t *testing.T) {
	rm := newTestManager()
	created := rm.CreateGuestRoom("guest001", "Movie Night", "", false)
	if created == nil {
		t.Fatal("expected guest room")
	}
	got := rm.OpenExistingRoom("guest001")
	if got == nil || got.ID != "guest001" {
		t.Fatalf("expected existing guest room, got %+v", got)
	}
}
