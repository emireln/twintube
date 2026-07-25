package room

import (
	"encoding/json"
	"math"
	"time"

	"twintube/internal/db"
)

func (c *Client) CanControlPlayback() bool {
	return c != nil && (c.IsHost || c.IsCohost)
}

func (c *Client) CanModerateQueue() bool {
	return c != nil && (c.IsHost || c.IsCohost)
}

func (c *Client) RoleName() string {
	if c == nil {
		return "viewer"
	}
	if c.IsHost {
		return "host"
	}
	if c.IsCohost {
		return "cohost"
	}
	return "viewer"
}

func (r *Room) CanClientAddToQueue(c *Client) bool {
	if c == nil {
		return false
	}
	if c.CanModerateQueue() {
		return true
	}
	r.mu.RLock()
	locked := r.QueueLocked
	r.mu.RUnlock()
	return !locked
}

func (r *Room) restoreCohostOnJoinLocked(client *Client) {
	if client.UserID != "" && r.CohostUserIDs[client.UserID] {
		client.IsCohost = true
	}
}

func (r *Room) promoteNewHostLocked() *Client {
	var bestCohost *Client
	for _, c := range r.Clients {
		if !c.IsCohost {
			continue
		}
		if bestCohost == nil || c.JoinedAt.Before(bestCohost.JoinedAt) {
			bestCohost = c
		}
	}
	if bestCohost != nil {
		return bestCohost
	}

	if r.OwnerID != "" {
		for _, c := range r.Clients {
			if c.UserID != "" && c.UserID == r.OwnerID {
				return c
			}
		}
	}

	var oldest *Client
	for _, c := range r.Clients {
		if oldest == nil || c.JoinedAt.Before(oldest.JoinedAt) {
			oldest = c
		}
	}
	return oldest
}

func (r *Room) GrantCohost(from *Client, targetClientID string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if from == nil || !from.IsHost {
		return "", false
	}
	target, ok := r.Clients[targetClientID]
	if !ok || target.IsHost {
		return "", false
	}
	target.IsCohost = true
	if target.UserID != "" {
		if r.CohostUserIDs == nil {
			r.CohostUserIDs = make(map[string]bool)
		}
		r.CohostUserIDs[target.UserID] = true
	}
	return target.Nickname, true
}

func (r *Room) RevokeCohost(from *Client, targetClientID string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if from == nil || !from.IsHost {
		return "", false
	}
	target, ok := r.Clients[targetClientID]
	if !ok {
		return "", false
	}
	target.IsCohost = false
	if target.UserID != "" && r.CohostUserIDs != nil {
		delete(r.CohostUserIDs, target.UserID)
	}
	return target.Nickname, true
}

func (r *Room) SetQueueLocked(from *Client, locked bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if from == nil || !from.CanModerateQueue() {
		return false
	}
	r.QueueLocked = locked
	return true
}

func (r *Room) QueueLockedValue() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.QueueLocked
}

func (r *Room) resetSkipVotesLocked() {
	r.SkipVotes = make(map[string]bool)
	r.SkipVideoID = r.State.VideoID
}

func (r *Room) ensureSkipVotesLocked() {
	if r.SkipVotes == nil || r.SkipVideoID != r.State.VideoID {
		r.resetSkipVotesLocked()
	}
}

func (r *Room) skipTallyLocked() (votes, needed int, hasVoted map[string]bool) {
	r.ensureSkipVotesLocked()
	votes = len(r.SkipVotes)
	n := len(r.Clients)
	needed = int(math.Ceil(float64(n) / 2.0))
	if needed < 1 {
		needed = 1
	}
	hasVoted = make(map[string]bool, len(r.SkipVotes))
	for id, v := range r.SkipVotes {
		hasVoted[id] = v
	}
	return votes, needed, hasVoted
}

// VoteSkip records a viewer vote. Returns (votes, needed, passed, ok).
func (r *Room) VoteSkip(from *Client) (votes, needed int, passed, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if from == nil {
		return 0, 0, false, false
	}
	r.ensureSkipVotesLocked()
	r.SkipVotes[from.ID] = true
	votes, needed, _ = r.skipTallyLocked()
	passed = votes >= needed
	if passed {
		r.resetSkipVotesLocked()
	}
	return votes, needed, passed, true
}

func (r *Room) SkipSnapshot() (votes, needed int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	votes, needed, _ = r.skipTallyLocked()
	return votes, needed
}

func (r *Room) ReorderPlaylist(order []string) ([]db.PlaylistItem, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(order) != len(r.Playlist) {
		return nil, false
	}
	byID := make(map[string]db.PlaylistItem, len(r.Playlist))
	for _, item := range r.Playlist {
		byID[item.ID] = item
	}
	next := make([]db.PlaylistItem, 0, len(order))
	seen := make(map[string]bool, len(order))
	for i, id := range order {
		item, exists := byID[id]
		if !exists || seen[id] {
			out := make([]db.PlaylistItem, len(r.Playlist))
			copy(out, r.Playlist)
			return out, false
		}
		seen[id] = true
		item.Position = i
		next = append(next, item)
	}
	if len(seen) != len(r.Playlist) {
		out := make([]db.PlaylistItem, len(r.Playlist))
		copy(out, r.Playlist)
		return out, false
	}
	r.Playlist = next
	out := make([]db.PlaylistItem, len(r.Playlist))
	copy(out, r.Playlist)
	return out, true
}

func (r *Room) renumberPlaylistLocked() {
	for i := range r.Playlist {
		r.Playlist[i].Position = i
	}
}

func (r *Room) BroadcastRoomMeta() {
	r.mu.Lock()
	votes, needed, _ := r.skipTallyLocked()
	locked := r.QueueLocked
	r.mu.Unlock()

	raw, _ := json.Marshal(map[string]interface{}{
		"queueLocked": locked,
		"skipVotes":   votes,
		"skipNeeded":  needed,
	})
	r.deliver(WSMessage{
		Action:  "ROOM_META",
		Payload: raw,
	})
}

func (r *Room) SendCorrectiveState(client *Client) {
	if client == nil {
		return
	}
	state := r.SnapshotState()
	nowMs := time.Now().UnixNano() / 1e6
	raw, _ := json.Marshal(map[string]interface{}{
		"videoId":         state.VideoID,
		"title":           state.Title,
		"status":          state.Status,
		"currentTime":     r.GetCalculatedTime(),
		"serverTimestamp": nowMs,
		"forceReload":     true,
		"platform":        state.Platform,
		"mediaKind":       state.MediaKind,
		"sourceUrl":       state.SourceURL,
		"seekable":        state.Seekable,
	})
	r.SendTo(client.ID, WSMessage{
		Action:    "STATE_UPDATE",
		Payload:   raw,
		Timestamp: nowMs,
	})
}

func (r *Room) roomMetaForClient(client *Client) map[string]interface{} {
	votes, needed, voted := r.skipTallyLocked()
	return map[string]interface{}{
		"queueLocked":        r.QueueLocked,
		"skipVotes":          votes,
		"skipNeeded":         needed,
		"hasSkipVoted":       voted[client.ID],
		"isCohost":           client.IsCohost,
		"canControlPlayback": client.CanControlPlayback(),
		"canModerateQueue":   client.CanModerateQueue(),
		"role":               client.RoleName(),
	}
}

func (r *Room) SetVoiceJoined(client *Client, joined bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if client == nil {
		return false
	}
	if joined {
		count := 0
		for _, c := range r.Clients {
			if c.VoiceJoined {
				count++
			}
		}
		if !client.VoiceJoined && count >= 6 {
			return false
		}
		client.VoiceJoined = true
		client.VoiceMuted = true
		client.VoiceSpeaking = false
	} else {
		client.VoiceJoined = false
		client.VoiceMuted = true
		client.VoiceSpeaking = false
	}
	return true
}

func (r *Room) SetVoiceStatus(client *Client, muted, speaking bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if client == nil || !client.VoiceJoined {
		return
	}
	client.VoiceMuted = muted
	client.VoiceSpeaking = speaking && !muted
}
