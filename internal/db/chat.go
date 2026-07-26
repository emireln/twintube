package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (d *DB) migrateChatColumns() error {
	alterations := []string{
		`ALTER TABLE messages ADD COLUMN reply_to_id VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN mentions TEXT DEFAULT '[]'`,
		`ALTER TABLE messages ADD COLUMN video_time DOUBLE PRECISION`,
	}
	if d.dbType == DBTypeSQLite {
		alterations = []string{
			`ALTER TABLE messages ADD COLUMN reply_to_id TEXT DEFAULT ''`,
			`ALTER TABLE messages ADD COLUMN mentions TEXT DEFAULT '[]'`,
			`ALTER TABLE messages ADD COLUMN video_time REAL`,
		}
	}
	for _, q := range alterations {
		if _, err := d.db.Exec(q); err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists") {
				continue
			}
		}
	}

	var reactionsDDL string
	if d.dbType == DBTypePostgres {
		reactionsDDL = `CREATE TABLE IF NOT EXISTS message_reactions (
			message_id VARCHAR(64) NOT NULL,
			room_id VARCHAR(64) NOT NULL,
			client_id VARCHAR(64) NOT NULL,
			user_id VARCHAR(64) DEFAULT '',
			emoji VARCHAR(32) NOT NULL,
			video_time DOUBLE PRECISION,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (message_id, client_id, emoji)
		);`
	} else {
		reactionsDDL = `CREATE TABLE IF NOT EXISTS message_reactions (
			message_id TEXT NOT NULL,
			room_id TEXT NOT NULL,
			client_id TEXT NOT NULL,
			user_id TEXT DEFAULT '',
			emoji TEXT NOT NULL,
			video_time REAL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (message_id, client_id, emoji)
		);`
	}
	if _, err := d.db.Exec(reactionsDDL); err != nil {
		return err
	}

	var cowatchDDL string
	if d.dbType == DBTypePostgres {
		cowatchDDL = `CREATE TABLE IF NOT EXISTS cowatchers (
			user_id VARCHAR(64) NOT NULL,
			peer_user_id VARCHAR(64) NOT NULL,
			last_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, peer_user_id)
		);`
	} else {
		cowatchDDL = `CREATE TABLE IF NOT EXISTS cowatchers (
			user_id TEXT NOT NULL,
			peer_user_id TEXT NOT NULL,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, peer_user_id)
		);`
	}
	_, err := d.db.Exec(cowatchDDL)
	return err
}

func mentionsToJSON(mentions []string) string {
	if len(mentions) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(mentions)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func mentionsFromJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func formatMessageID(id int64) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", id)
}

// SaveChatMessage inserts a chat row and returns its string id.
func (d *DB) SaveChatMessage(roomID, userID, nickname, content string, isSystem bool, replyToID string, mentions []string, videoTime *float64) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.roomIsOwnedLocked(roomID) {
		return "", nil
	}

	mentionsJSON := mentionsToJSON(mentions)

	if d.dbType == DBTypePostgres {
		query := d.Rebind(`INSERT INTO messages (room_id, user_id, nickname, content, is_system, reply_to_id, mentions, video_time, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP) RETURNING id;`)
		var insertedID int64
		err := d.db.QueryRow(query, roomID, userID, nickname, content, isSystem, replyToID, mentionsJSON, videoTime).Scan(&insertedID)
		if err != nil {
			return "", err
		}
		return formatMessageID(insertedID), nil
	}

	query := d.Rebind(`INSERT INTO messages (room_id, user_id, nickname, content, is_system, reply_to_id, mentions, video_time, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP);`)
	res, err := d.db.Exec(query, roomID, userID, nickname, content, isSystem, replyToID, mentionsJSON, videoTime)
	if err != nil {
		return "", err
	}
	insertedID, err := res.LastInsertId()
	if err != nil {
		return "", err
	}
	return formatMessageID(insertedID), nil
}

func (d *DB) LoadChatHistory(roomID string, limit int) ([]ChatMessage, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`
		SELECT m.id, m.nickname, COALESCE(m.user_id, ''), COALESCE(u.avatar_url, ''), m.content, m.is_system,
			m.reply_to_id, COALESCE(m.mentions, '[]'), m.video_time, m.created_at
		FROM messages m
		LEFT JOIN users u ON u.id = m.user_id
		WHERE m.room_id = ?
		ORDER BY m.id DESC
		LIMIT ?;`)
	rows, err := d.db.Query(query, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ChatMessage
	var ids []string
	for rows.Next() {
		var msg ChatMessage
		var dbID int64
		var isSys bool
		var replyToID string
		var mentionsRaw string
		var videoTime *float64
		var createdAt time.Time

		if err := rows.Scan(&dbID, &msg.Nickname, &msg.UserID, &msg.AvatarURL, &msg.Content, &isSys,
			&replyToID, &mentionsRaw, &videoTime, &createdAt); err != nil {
			continue
		}

		msg.ID = formatMessageID(dbID)
		msg.Type = "chat"
		msg.IsSystem = isSys
		msg.ReplyToID = strings.TrimSpace(replyToID)
		msg.Mentions = mentionsFromJSON(mentionsRaw)
		msg.VideoTime = videoTime
		msg.Timestamp = createdAt.Format("15:04")
		messages = append(messages, msg)
		ids = append(ids, msg.ID)
	}

	reactionMap, _ := d.loadReactionCountsLocked(roomID, ids)

	for i := range messages {
		if counts, ok := reactionMap[messages[i].ID]; ok && len(counts) > 0 {
			messages[i].Reactions = counts
		}
		if messages[i].ReplyToID != "" {
			messages[i].ReplyToNick, messages[i].ReplyToText = d.lookupReplyPreviewLocked(roomID, messages[i].ReplyToID)
		}
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (d *DB) lookupReplyPreviewLocked(roomID, replyToID string) (string, string) {
	query := d.Rebind(`SELECT nickname, content FROM messages WHERE room_id = ? AND id = ? LIMIT 1;`)
	var nick, content string
	var dbID int64
	if _, err := fmt.Sscan(replyToID, &dbID); err != nil {
		return "", ""
	}
	err := d.db.QueryRow(query, roomID, dbID).Scan(&nick, &content)
	if err != nil {
		return "", ""
	}
	if len(content) > 80 {
		content = content[:80] + "…"
	}
	return nick, content
}

func (d *DB) loadReactionCountsLocked(roomID string, messageIDs []string) (map[string]map[string]int, error) {
	out := make(map[string]map[string]int)
	if len(messageIDs) == 0 {
		return out, nil
	}

	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, 0, len(messageIDs)+1)
	args = append(args, roomID)
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := d.Rebind(fmt.Sprintf(`
		SELECT message_id, emoji, COUNT(*) AS c
		FROM message_reactions
		WHERE room_id = ? AND message_id IN (%s)
		GROUP BY message_id, emoji;`, strings.Join(placeholders, ",")))

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var msgID, emoji string
		var count int
		if err := rows.Scan(&msgID, &emoji, &count); err != nil {
			continue
		}
		if out[msgID] == nil {
			out[msgID] = make(map[string]int)
		}
		out[msgID][emoji] = count
	}
	return out, nil
}

// ToggleChatReaction adds or removes a reaction; returns new counts and whether it was added.
func (d *DB) ToggleChatReaction(roomID, messageID, clientID, userID, emoji string, videoTime *float64) (map[string]int, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.roomIsOwnedLocked(roomID) {
		return nil, false, nil
	}

	check := d.Rebind(`SELECT 1 FROM message_reactions WHERE message_id = ? AND client_id = ? AND emoji = ? LIMIT 1;`)
	var exists int
	err := d.db.QueryRow(check, messageID, clientID, emoji).Scan(&exists)
	added := err != nil

	if added {
		insert := d.Rebind(`INSERT INTO message_reactions (message_id, room_id, client_id, user_id, emoji, video_time, created_at)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP);`)
		if _, err := d.db.Exec(insert, messageID, roomID, clientID, userID, emoji, videoTime); err != nil {
			return nil, false, err
		}
	} else {
		del := d.Rebind(`DELETE FROM message_reactions WHERE message_id = ? AND client_id = ? AND emoji = ?;`)
		if _, err := d.db.Exec(del, messageID, clientID, emoji); err != nil {
			return nil, false, err
		}
	}

	counts, err := d.loadReactionCountsLocked(roomID, []string{messageID})
	if err != nil {
		return nil, added, err
	}
	if counts[messageID] == nil {
		return map[string]int{}, added, nil
	}
	return counts[messageID], added, nil
}

func (d *DB) LoadClientReactions(roomID, clientID string, messageIDs []string) map[string][]string {
	d.mu.Lock()
	defer d.mu.Unlock()

	out := make(map[string][]string)
	if len(messageIDs) == 0 {
		return out
	}

	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, 0, len(messageIDs)+2)
	args = append(args, roomID, clientID)
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := d.Rebind(fmt.Sprintf(`
		SELECT message_id, emoji FROM message_reactions
		WHERE room_id = ? AND client_id = ? AND message_id IN (%s);`, strings.Join(placeholders, ",")))

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var msgID, emoji string
		if err := rows.Scan(&msgID, &emoji); err != nil {
			continue
		}
		out[msgID] = append(out[msgID], emoji)
	}
	return out
}

// RecordCoWatchers links authenticated users who shared a room session.
func (d *DB) RecordCoWatchers(userIDs []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	unique := make([]string, 0, len(userIDs))
	seen := make(map[string]bool)
	for _, id := range userIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	if len(unique) < 2 {
		return
	}

	query := d.Rebind(`INSERT INTO cowatchers (user_id, peer_user_id, last_seen)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, peer_user_id) DO UPDATE SET last_seen = CURRENT_TIMESTAMP;`)
	if d.dbType == DBTypePostgres {
		query = d.Rebind(`INSERT INTO cowatchers (user_id, peer_user_id, last_seen)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT (user_id, peer_user_id) DO UPDATE SET last_seen = EXCLUDED.last_seen;`)
	} else {
		query = d.Rebind(`INSERT INTO cowatchers (user_id, peer_user_id, last_seen)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_id, peer_user_id) DO UPDATE SET last_seen = CURRENT_TIMESTAMP;`)
	}

	for _, a := range unique {
		for _, b := range unique {
			if a == b {
				continue
			}
			_, _ = d.db.Exec(query, a, b)
		}
	}
}

// GetCoWatcherUserIDs returns peers the user has watched with recently.
func (d *DB) GetCoWatcherUserIDs(userID string) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}

	query := d.Rebind(`SELECT peer_user_id FROM cowatchers WHERE user_id = ? ORDER BY last_seen DESC LIMIT 100;`)
	rows, err := d.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []string
	for rows.Next() {
		var peer string
		if err := rows.Scan(&peer); err != nil {
			continue
		}
		peers = append(peers, peer)
	}
	return peers, nil
}

// DeleteEphemeralRoomData should also clear reactions for the room.
func (d *DB) deleteMessageReactionsForRoomLocked(roomID string) {
	query := d.Rebind(`DELETE FROM message_reactions WHERE room_id = ?;`)
	_, _ = d.db.Exec(query, roomID)
}
