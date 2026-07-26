package room

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"twintube/internal/db"
	"twintube/internal/utils"
)

const chatLogMax = 100

var mentionPattern = regexp.MustCompile(`@([A-Za-z0-9_\-.]{1,25})`)

var allowedChatEmojis = map[string]bool{
	"😀": true, "😂": true, "😍": true, "🔥": true, "👏": true, "👍": true,
	"🎉": true, "🚀": true, "❤️": true, "🥳": true, "😎": true, "💡": true,
	"🍿": true, "🎬": true, "🎵": true, "💯": true, "🙌": true, "💩": true,
	"🤩": true, "✨": true, "😢": true, "😮": true, "👀": true, "💀": true,
}

type chatMessageRef struct {
	Nickname string
	Content  string
}

type HypeBurst struct {
	Emoji     string  `json:"emoji"`
	VideoTime float64 `json:"videoTime"`
	Count     int     `json:"count"`
	Label     string  `json:"label"`
}

type hypeBucket struct {
	emoji     string
	videoTime float64
	count     int
	lastAt    time.Time
}

// AllowedChatEmoji reports whether an emoji can be used on chat messages.
func AllowedChatEmoji(emoji string) bool {
	return allowedChatEmojis[emoji]
}

func truncateChatPreview(content string) string {
	content = strings.TrimSpace(content)
	if len(content) <= 80 {
		return content
	}
	return content[:80] + "…"
}

func (r *Room) rememberChatMessage(msg db.ChatMessage) {
	if msg.ID == "" {
		return
	}
	if r.chatByID == nil {
		r.chatByID = make(map[string]chatMessageRef)
	}
	r.chatByID[msg.ID] = chatMessageRef{
		Nickname: msg.Nickname,
		Content:  msg.Content,
	}
	r.chatLog = append(r.chatLog, msg)
	if len(r.chatLog) > chatLogMax {
		removed := r.chatLog[0]
		delete(r.chatByID, removed.ID)
		r.chatLog = r.chatLog[1:]
	}
}

func (r *Room) chatHistorySnapshot() []db.ChatMessage {
	out := make([]db.ChatMessage, len(r.chatLog))
	copy(out, r.chatLog)
	return out
}

func (r *Room) hydrateChatFromDB() {
	if db.Database == nil || r.OwnerID == "" || len(r.chatLog) > 0 {
		return
	}
	history, err := db.Database.LoadChatHistory(r.ID, chatLogMax)
	if err != nil || len(history) == 0 {
		return
	}
	for _, msg := range history {
		r.rememberChatMessage(msg)
	}
}

func (r *Room) nicknamesInRoom() map[string]string {
	out := make(map[string]string)
	for _, c := range r.Clients {
		nick := strings.TrimSpace(c.Nickname)
		if nick == "" {
			continue
		}
		out[strings.ToLower(nick)] = nick
	}
	return out
}

func ParseMentions(content string, nicknames map[string]string) []string {
	return parseMentions(content, nicknames)
}

func parseMentions(content string, nicknames map[string]string) []string {
	if len(nicknames) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var mentions []string
	matches := mentionPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		key := strings.ToLower(m[1])
		if canonical, ok := nicknames[key]; ok && !seen[canonical] {
			seen[canonical] = true
			mentions = append(mentions, canonical)
		}
	}
	return mentions
}

func (r *Room) PostUserChat(client *Client, content, replyToID string, videoTime *float64) (db.ChatMessage, []*Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.chatByID == nil {
		r.chatByID = make(map[string]chatMessageRef)
	}
	if r.chatReacts == nil {
		r.chatReacts = make(map[string]map[string]map[string]bool)
	}

	r.hydrateChatFromDB()

	replyToID = strings.TrimSpace(replyToID)
	var replyNick, replyText string
	if replyToID != "" {
		if ref, ok := r.chatByID[replyToID]; ok {
			replyNick = ref.Nickname
			replyText = truncateChatPreview(ref.Content)
		} else {
			replyToID = ""
		}
	}

	mentions := parseMentions(content, r.nicknamesInRoom())

	msg := db.ChatMessage{
		Type:        "chat",
		Nickname:    client.Nickname,
		UserID:      client.UserID,
		AvatarURL:   client.AvatarURL,
		Content:     content,
		IsSystem:    false,
		Timestamp:   time.Now().Format("15:04"),
		ReplyToID:   replyToID,
		ReplyToNick: replyNick,
		ReplyToText: replyText,
		Mentions:    mentions,
		VideoTime:   videoTime,
		Reactions:   map[string]int{},
	}

	if db.Database != nil && r.OwnerID != "" {
		id, err := db.Database.SaveChatMessage(r.ID, client.UserID, client.Nickname, content, false, replyToID, mentions, videoTime)
		if err == nil && id != "" {
			msg.ID = id
		}
	}
	if msg.ID == "" {
		msg.ID = "g" + utils.GenerateRandomID(8)
	}

	r.rememberChatMessage(msg)

	var mentioned []*Client
	if len(mentions) > 0 {
		want := make(map[string]bool)
		for _, m := range mentions {
			want[strings.ToLower(m)] = true
		}
		for _, c := range r.Clients {
			if c.ID == client.ID {
				continue
			}
			if want[strings.ToLower(c.Nickname)] {
				mentioned = append(mentioned, c)
			}
		}
	}

	return msg, mentioned
}

func (r *Room) PostSystemChat(content string) db.ChatMessage {
	msg := db.ChatMessage{
		ID:        "s" + utils.GenerateRandomID(6),
		Type:      "chat",
		Nickname:  "System",
		Content:   content,
		IsSystem:  true,
		Timestamp: time.Now().Format("15:04"),
	}

	if db.Database != nil && r.IsPersistent() {
		id, err := db.Database.SaveChatMessage(r.ID, "", "System", content, true, "", nil, nil)
		if err == nil && id != "" {
			msg.ID = id
		}
	}

	r.mu.Lock()
	if r.chatByID == nil {
		r.chatByID = make(map[string]chatMessageRef)
	}
	r.rememberChatMessage(msg)
	r.mu.Unlock()
	return msg
}

type ChatReactionUpdate struct {
	MessageID string         `json:"messageId"`
	Reactions map[string]int `json:"reactions"`
	Emoji     string         `json:"emoji"`
	Added     bool           `json:"added"`
	ClientID  string         `json:"clientId"`
	VideoTime *float64       `json:"videoTime,omitempty"`
}

func (r *Room) ToggleChatReaction(client *Client, messageID, emoji string, videoTime *float64) (ChatReactionUpdate, *HypeBurst) {
	update := ChatReactionUpdate{MessageID: messageID, Emoji: emoji, ClientID: client.ID}
	if !AllowedChatEmoji(emoji) {
		return update, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.chatByID == nil {
		return update, nil
	}
	if _, ok := r.chatByID[messageID]; !ok {
		return update, nil
	}

	if r.chatReacts == nil {
		r.chatReacts = make(map[string]map[string]map[string]bool)
	}
	if r.chatReacts[messageID] == nil {
		r.chatReacts[messageID] = make(map[string]map[string]bool)
	}
	if r.chatReacts[messageID][emoji] == nil {
		r.chatReacts[messageID][emoji] = make(map[string]bool)
	}

	added := !r.chatReacts[messageID][emoji][client.ID]
	if added {
		r.chatReacts[messageID][emoji][client.ID] = true
	} else {
		delete(r.chatReacts[messageID][emoji], client.ID)
	}

	counts := aggregateReactionCounts(r.chatReacts[messageID])
	update.Reactions = counts
	update.Added = added
	update.VideoTime = videoTime

	for i := range r.chatLog {
		if r.chatLog[i].ID == messageID {
			r.chatLog[i].Reactions = counts
			break
		}
	}

	if db.Database != nil && r.OwnerID != "" {
		dbCounts, dbAdded, err := db.Database.ToggleChatReaction(r.ID, messageID, client.ID, client.UserID, emoji, videoTime)
		if err == nil {
			update.Reactions = dbCounts
			update.Added = dbAdded
			syncReactionMapsFromCounts(r.chatReacts[messageID], messageID, client.ID, emoji, dbAdded)
			counts = dbCounts
		}
	}

	var hype *HypeBurst
	if added && videoTime != nil && *videoTime >= 0 {
		hype = r.trackHypeLocked(emoji, *videoTime)
	}

	return update, hype
}

func aggregateReactionCounts(byEmoji map[string]map[string]bool) map[string]int {
	counts := make(map[string]int)
	for e, clients := range byEmoji {
		if len(clients) > 0 {
			counts[e] = len(clients)
		}
	}
	return counts
}

func syncReactionMapsFromCounts(byEmoji map[string]map[string]bool, messageID, clientID, emoji string, added bool) {
	if byEmoji[emoji] == nil {
		byEmoji[emoji] = make(map[string]bool)
	}
	if added {
		byEmoji[emoji][clientID] = true
	} else {
		delete(byEmoji[emoji], clientID)
	}
}

func (r *Room) trackHypeLocked(emoji string, videoTime float64) *HypeBurst {
	now := time.Now()
	bucketKey := int(videoTime)
	total := 1
	for i := len(r.hypeBuckets) - 1; i >= 0; i-- {
		b := r.hypeBuckets[i]
		if now.Sub(b.lastAt) > 3*time.Second {
			continue
		}
		if int(b.videoTime) == bucketKey {
			total += b.count
		}
	}

	r.hypeBuckets = append(r.hypeBuckets, hypeBucket{
		emoji:     emoji,
		videoTime: videoTime,
		count:     1,
		lastAt:    now,
	})
	if len(r.hypeBuckets) > 64 {
		r.hypeBuckets = r.hypeBuckets[len(r.hypeBuckets)-64:]
	}

	if total >= 5 {
		return &HypeBurst{
			Emoji:     emoji,
			VideoTime: videoTime,
			Count:     total,
			Label:     FormatVideoTimeLabel(videoTime),
		}
	}
	return nil
}

func FormatVideoTimeLabel(videoTime float64) string {
	total := int(videoTime)
	if total < 0 {
		total = 0
	}
	mins := total / 60
	secs := total % 60
	if mins > 0 {
		return fmt.Sprintf("%d:%02d", mins, secs)
	}
	return fmt.Sprintf("0:%02d", secs)
}

// BroadcastChatMessage fans out a user/system chat line without the Broadcast channel.
func (r *Room) BroadcastChatMessage(msg db.ChatMessage) {
	raw := MarshalChatMessage(msg)
	r.deliver(WSMessage{Action: "CHAT_MESSAGE", Payload: raw})
}

func (r *Room) BroadcastChatReaction(update ChatReactionUpdate) {
	raw, _ := json.Marshal(update)
	r.deliver(WSMessage{Action: "CHAT_REACTION_UPDATE", Payload: raw})
}

func (r *Room) BroadcastHypeBurst(hype HypeBurst) {
	raw, _ := json.Marshal(hype)
	r.deliver(WSMessage{Action: "HYPE_BURST", Payload: raw})
}

// MarshalChatMessage JSON-encodes a chat message for WS payloads.
func MarshalChatMessage(msg db.ChatMessage) json.RawMessage {
	raw, _ := json.Marshal(msg)
	return raw
}

// RecordCoWatchersFromRoom persists co-watcher pairs for authenticated viewers.
func (r *Room) RecordCoWatchersFromRoom() {
	if db.Database == nil {
		return
	}
	r.mu.RLock()
	ids := make([]string, 0, len(r.Clients))
	for _, c := range r.Clients {
		if c.UserID != "" {
			ids = append(ids, c.UserID)
		}
	}
	r.mu.RUnlock()
	if len(ids) >= 2 {
		db.Database.RecordCoWatchers(ids)
	}
}
