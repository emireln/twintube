package room

import (
	"encoding/json"
	"testing"
	"time"
)

// waitForAction reads a client's queued messages until the wanted action arrives.
func waitForAction(t *testing.T, ch <-chan WSMessage, action string) WSMessage {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed before %s arrived", action)
			}
			if msg.Action == action {
				return msg
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", action)
		}
	}
}

// within fails the test if fn does not return promptly, which is how a join or
// leave would hang on a stopped run loop.
func within(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s blocked", what)
	}
}

func newLiveRoom(t *testing.T, id string) (*RoomManager, *Room) {
	t.Helper()
	rm := &RoomManager{rooms: make(map[string]*Room)}
	room := rm.newRoomShell(id, "Lifecycle")
	rm.rooms[id] = room
	go room.Run()
	return rm, room
}

func TestEndRoomDisconnectsParticipantsAndStopsRun(t *testing.T) {
	rm, room := newLiveRoom(t, "lifecycle1")

	client := &Client{ID: "c1", Nickname: "Ana", Send: make(chan WSMessage, 32)}
	if !room.Join(client) {
		t.Fatal("join should succeed for a live room")
	}
	waitForAction(t, client.Send, "INIT_STATE")

	rm.EndRoom(room.ID, "Room not found.")
	if rm.GetRoom(room.ID) != nil {
		t.Fatal("ended room should be gone from the manager")
	}

	errMsg := waitForAction(t, client.Send, "ERROR")
	var payload map[string]string
	if err := json.Unmarshal(errMsg.Payload, &payload); err != nil || payload["message"] != "Room not found." {
		t.Fatalf("participant should learn the room is gone, got %s", errMsg.Payload)
	}

	// The read loop calls Leave once the socket closes. That must not block, and it
	// lets the run loop stop, because the room is closed and empty.
	within(t, "Leave on a closed room", func() { room.Leave(client) })

	joined := true
	within(t, "Join on a stopped room", func() {
		joined = room.Join(&Client{ID: "c2", Send: make(chan WSMessage, 1)})
	})
	if joined {
		t.Fatal("join must fail once the room has stopped")
	}
}

func TestEndEmptyRoomStopsRunLoop(t *testing.T) {
	rm, room := newLiveRoom(t, "lifecycle2")

	// Nobody ever joined, so nothing would send Unregister. Closing must still stop the loop.
	rm.EndRoom(room.ID, "Room not found.")

	joined := true
	within(t, "Join on an emptied, closed room", func() {
		joined = room.Join(&Client{ID: "late", Send: make(chan WSMessage, 1)})
	})
	if joined {
		t.Fatal("a closed room with no participants must not accept joins")
	}
	within(t, "Publish on a stopped room", func() {
		for i := 0; i < 200; i++ {
			room.Publish(WSMessage{Action: "STATE_UPDATE"})
		}
	})
}

func TestEndRoomIsIdempotent(t *testing.T) {
	rm, room := newLiveRoom(t, "lifecycle3")
	rm.EndRoom(room.ID, "Room not found.")
	within(t, "second EndRoom", func() { rm.EndRoom(room.ID, "Room not found.") })
}
