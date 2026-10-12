package room

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"twintube/internal/db"
)

func TestTruncateChatPreviewKeepsCharacters(t *testing.T) {
	got := truncateChatPreview(strings.Repeat("é", 100))
	if !utf8.ValidString(got) {
		t.Fatalf("preview split a character: %q", got)
	}
	if n := utf8.RuneCountInString(got); n != 81 {
		t.Fatalf("want 80 characters plus an ellipsis, got %d characters", n)
	}
}

func TestMentionNotifySnippetKeepsCharacters(t *testing.T) {
	client := &Client{Send: make(chan WSMessage, 1)}
	SendMentionNotify(client, "room1", db.ChatMessage{ID: "m1", Nickname: "Ana", Content: strings.Repeat("ç", 200)})

	msg := <-client.Send
	var payload MentionNotifyPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if !utf8.ValidString(payload.Content) {
		t.Fatalf("mention snippet split a character: %q", payload.Content)
	}
}

func TestMomentLabelKeepsCharacters(t *testing.T) {
	r := &Room{
		Permissions: DefaultRoomPermissions(),
		State:       VideoState{VideoID: "dQw4w9WgXcQ", Platform: "youtube", MediaKind: "vod", Seekable: true},
	}
	moment, ok := r.SubmitMoment(&Client{ID: "c1", IsHost: true}, 12, strings.Repeat("ç", 200))
	if !ok {
		t.Fatal("host moment should be accepted")
	}
	if n := utf8.RuneCountInString(moment.Label); n != 120 || !utf8.ValidString(moment.Label) {
		t.Fatalf("want 120 valid characters, got %d", n)
	}
}
