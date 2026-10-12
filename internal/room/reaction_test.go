package room

import "testing"

// Reactions for unknown messages or disallowed emoji used to be broadcast to every
// client with null counts. They must now be refused, with nothing to broadcast.
func TestToggleChatReactionRefusesUnknownMessagesAndEmoji(t *testing.T) {
	r := &Room{
		chatByID:   map[string]chatMessageRef{"m1": {Nickname: "Ana", Content: "hi"}},
		chatReacts: make(map[string]map[string]map[string]bool),
	}
	client := &Client{ID: "c1"}

	if _, _, ok := r.ToggleChatReaction(client, "missing", "👍", nil); ok {
		t.Fatal("reaction to an unknown message must not be accepted")
	}
	if _, _, ok := r.ToggleChatReaction(client, "m1", "🐍", nil); ok {
		t.Fatal("reaction with a disallowed emoji must not be accepted")
	}

	update, _, ok := r.ToggleChatReaction(client, "m1", "👍", nil)
	if !ok || update.Reactions["👍"] != 1 {
		t.Fatalf("valid reaction should be accepted with count 1, got ok=%v reactions=%v", ok, update.Reactions)
	}

	update, _, ok = r.ToggleChatReaction(client, "m1", "👍", nil)
	if !ok || update.Added || len(update.Reactions) != 0 {
		t.Fatalf("second press should remove the reaction, got ok=%v added=%v reactions=%v", ok, update.Added, update.Reactions)
	}
}
