package room

import (
	"sync"
	"testing"
	"time"
)

func TestGuestRoomSurvivesGracePeriod(t *testing.T) {
	prev := GuestRoomGracePeriod
	GuestRoomGracePeriod = 80 * time.Millisecond
	t.Cleanup(func() { GuestRoomGracePeriod = prev })

	rm := newTestManager()
	room := rm.CreateGuestRoom("grace01", "Grace Test", "", false)
	client := &Client{ID: "c1", Nickname: "Guest", JoinedAt: time.Now(), Send: make(chan WSMessage, 4)}
	room.Register <- client
	time.Sleep(20 * time.Millisecond)

	room.Unregister <- client
	time.Sleep(30 * time.Millisecond)

	if rm.GetRoom("grace01") == nil {
		t.Fatal("guest room should stay alive during grace period")
	}

	time.Sleep(120 * time.Millisecond)

	if rm.GetRoom("grace01") != nil {
		t.Fatal("guest room should be removed after grace period")
	}
}

func TestGuestRoomGraceCancelledOnRejoin(t *testing.T) {
	prev := GuestRoomGracePeriod
	GuestRoomGracePeriod = 200 * time.Millisecond
	t.Cleanup(func() { GuestRoomGracePeriod = prev })

	rm := newTestManager()
	room := rm.CreateGuestRoom("grace02", "Grace Rejoin", "", false)
	c1 := &Client{ID: "c1", Nickname: "A", JoinedAt: time.Now(), Send: make(chan WSMessage, 4)}
	room.Register <- c1
	time.Sleep(20 * time.Millisecond)
	room.Unregister <- c1
	time.Sleep(50 * time.Millisecond)

	c2 := &Client{ID: "c2", Nickname: "B", JoinedAt: time.Now(), Send: make(chan WSMessage, 4)}
	room.Register <- c2
	time.Sleep(250 * time.Millisecond)

	if rm.GetRoom("grace02") == nil {
		t.Fatal("guest room should survive when someone rejoins before grace expires")
	}
}

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
