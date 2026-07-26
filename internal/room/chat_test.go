package room

import (
	"encoding/json"
	"testing"
	"time"

	"twintube/internal/db"
)

func TestPostUserChatBroadcast(t *testing.T) {
	r := Manager.CreateGuestRoom("testroom", "Test", "", false)
	go r.Run()

	client := &Client{
		ID:       "c1",
		Nickname: "Tester",
		Send:     make(chan WSMessage, 16),
	}
	r.Register <- client

	deadline := time.After(2 * time.Second)
waitInit:
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for registration")
		case msg := <-client.Send:
			if msg.Action == "INIT_STATE" {
				break waitInit
			}
		}
	}

	msg, _ := r.PostUserChat(client, "hello unit test", "", nil)
	r.BroadcastChatMessage(msg)

	got := false
	readDeadline := time.After(2 * time.Second)
	for !got {
		select {
		case <-readDeadline:
			t.Fatal("timed out waiting for CHAT_MESSAGE broadcast")
		case out := <-client.Send:
			if out.Action != "CHAT_MESSAGE" {
				continue
			}
			var chat db.ChatMessage
			if err := json.Unmarshal(out.Payload, &chat); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if chat.Content == "hello unit test" && chat.Nickname == "Tester" && !chat.IsSystem {
				got = true
			}
		}
	}

	close(r.stopCh)
}

func TestPostUserChatConcurrentWithRegister(t *testing.T) {
	r := Manager.CreateGuestRoom("conctest", "T", "", false)
	go r.Run()

	client := &Client{
		ID:       "c1",
		Nickname: "Tester",
		IsGuest:  true,
		Send:     make(chan WSMessage, 64),
	}

	registerDone := make(chan struct{})
	go func() {
		r.Register <- client
		close(registerDone)
	}()

	chatDone := make(chan struct{})
	go func() {
		time.Sleep(5 * time.Millisecond)
		msg, _ := r.PostUserChat(client, "hello concurrent", "", nil)
		r.BroadcastChatMessage(msg)
		close(chatDone)
	}()

	select {
	case <-registerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("register blocked")
	}
	select {
	case <-chatDone:
	case <-time.After(3 * time.Second):
		t.Fatal("post user chat blocked")
	}

	close(r.stopCh)
}

func TestPostUserChatOwnedRoomNoDeadlock(t *testing.T) {
	r := Manager.CreateGuestRoom("ownedchat", "Owned", "", false)
	r.OwnerID = "local-user-1"
	go r.Run()

	client := &Client{
		ID:       "c1",
		Nickname: "Tester",
		UserID:   "local-user-1",
		Send:     make(chan WSMessage, 16),
	}
	r.Register <- client

	deadline := time.After(2 * time.Second)
waitInit:
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for registration")
		case msg := <-client.Send:
			if msg.Action == "INIT_STATE" {
				break waitInit
			}
		}
	}

	done := make(chan struct{})
	go func() {
		msg, _ := r.PostUserChat(client, "hello owned room", "", nil)
		r.BroadcastChatMessage(msg)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("PostUserChat deadlocked on owned room")
	}

	close(r.stopCh)
}
