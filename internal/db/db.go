package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type DBType string

const (
	DBTypePostgres DBType = "postgres"
	DBTypeSQLite   DBType = "sqlite"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	AvatarURL    string    `json:"avatarUrl"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ChatMessage struct {
	Type      string `json:"type"`
	Nickname  string `json:"nickname"`
	Content   string `json:"content"`
	IsSystem  bool   `json:"isSystem"`
	Timestamp string `json:"timestamp"`
}

type PlaylistItem struct {
	ID           string `json:"id"`
	RoomID       string `json:"roomId"`
	VideoID      string `json:"videoId"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	ThumbnailURL string `json:"thumbnailUrl"`
	Position     int    `json:"position"`
	AddedBy      string `json:"addedBy"`
}

type DB struct {
	db     *sql.DB
	dbType DBType
	mu     sync.Mutex
}

var Database *DB

// InitDB initializes the database (PostgreSQL or SQLite based on configuration)
func InitDB(defaultSQLitePath string) (*DB, error) {
	var dbType DBType
	var connStr string

	dbTypeEnv := os.Getenv("DB_TYPE")
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL != "" || strings.ToLower(dbTypeEnv) == "postgres" {
		dbType = DBTypePostgres
		if databaseURL != "" {
			connStr = databaseURL
		} else {
			host := os.Getenv("POSTGRES_HOST")
			if host == "" { host = "localhost" }
			port := os.Getenv("POSTGRES_PORT")
			if port == "" { port = "5432" }
			user := os.Getenv("POSTGRES_USER")
			if user == "" { user = "twintube" }
			pass := os.Getenv("POSTGRES_PASSWORD")
			if pass == "" { pass = "postgres" }
			dbname := os.Getenv("POSTGRES_DB")
			if dbname == "" { dbname = "twintube" }
			sslmode := os.Getenv("POSTGRES_SSLMODE")
			if sslmode == "" { sslmode = "disable" }

			connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
				host, port, user, pass, dbname, sslmode)
		}
	} else {
		dbType = DBTypeSQLite
		connStr = defaultSQLitePath
	}

	driverName := "sqlite"
	if dbType == DBTypePostgres {
		driverName = "postgres"
	}

	sqlDB, err := sql.Open(driverName, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s database: %w", dbType, err)
	}

	if dbType == DBTypeSQLite {
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping %s database: %w", dbType, err)
	}

	dbWrapper := &DB{
		db:     sqlDB,
		dbType: dbType,
	}

	if err := dbWrapper.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	Database = dbWrapper
	log.Printf("[DB] Successfully initialized %s database connection", strings.ToUpper(string(dbType)))
	return dbWrapper, nil
}

func (d *DB) Rebind(query string) string {
	if d.dbType != DBTypePostgres {
		return query
	}

	var result strings.Builder
	paramIdx := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			fmt.Fprintf(&result, "$%d", paramIdx)
			paramIdx++
		} else {
			result.WriteByte(query[i])
		}
	}
	return result.String()
}

func (d *DB) createTables() error {
	queries := []string{}

	if d.dbType == DBTypePostgres {
		queries = []string{
			`CREATE TABLE IF NOT EXISTS users (
				id VARCHAR(64) PRIMARY KEY,
				username VARCHAR(64) UNIQUE NOT NULL,
				email VARCHAR(255) UNIQUE NOT NULL,
				password_hash TEXT NOT NULL,
				avatar_url TEXT DEFAULT '',
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			);`,

			`CREATE TABLE IF NOT EXISTS rooms (
				id VARCHAR(64) PRIMARY KEY,
				name VARCHAR(255) DEFAULT '',
				is_private BOOLEAN DEFAULT FALSE,
				password_hash TEXT DEFAULT '',
				host_id VARCHAR(64) NOT NULL,
				current_video_id VARCHAR(64) DEFAULT 'dQw4w9WgXcQ',
				current_status VARCHAR(32) DEFAULT 'PAUSED',
				"current_time" DOUBLE PRECISION DEFAULT 0.0,
				updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			);`,

			`CREATE TABLE IF NOT EXISTS messages (
				id BIGSERIAL PRIMARY KEY,
				room_id VARCHAR(64) NOT NULL,
				user_id VARCHAR(64) DEFAULT '',
				nickname VARCHAR(64) NOT NULL,
				content TEXT NOT NULL,
				is_system BOOLEAN DEFAULT FALSE,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			);`,

			`CREATE TABLE IF NOT EXISTS playlist_items (
				id VARCHAR(64) PRIMARY KEY,
				room_id VARCHAR(64) NOT NULL,
				video_id VARCHAR(64) NOT NULL,
				title TEXT NOT NULL,
				author VARCHAR(255) DEFAULT '',
				thumbnail_url TEXT DEFAULT '',
				position INT NOT NULL,
				added_by VARCHAR(64) NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			);`,
		}
	} else {
		queries = []string{
			`CREATE TABLE IF NOT EXISTS users (
				id TEXT PRIMARY KEY,
				username TEXT UNIQUE NOT NULL,
				email TEXT UNIQUE NOT NULL,
				password_hash TEXT NOT NULL,
				avatar_url TEXT DEFAULT '',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);`,

			`CREATE TABLE IF NOT EXISTS rooms (
				id TEXT PRIMARY KEY,
				name TEXT DEFAULT '',
				is_private INTEGER DEFAULT 0,
				password_hash TEXT DEFAULT '',
				host_id TEXT NOT NULL,
				current_video_id TEXT DEFAULT 'dQw4w9WgXcQ',
				current_status TEXT DEFAULT 'PAUSED',
				current_time REAL DEFAULT 0.0,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);`,

			`CREATE TABLE IF NOT EXISTS messages (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				room_id TEXT NOT NULL,
				user_id TEXT DEFAULT '',
				nickname TEXT NOT NULL,
				content TEXT NOT NULL,
				is_system INTEGER DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);`,

			`CREATE TABLE IF NOT EXISTS playlist_items (
				id TEXT PRIMARY KEY,
				room_id TEXT NOT NULL,
				video_id TEXT NOT NULL,
				title TEXT NOT NULL,
				author TEXT DEFAULT '',
				thumbnail_url TEXT DEFAULT '',
				position INTEGER NOT NULL,
				added_by TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);`,
		}
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return err
		}
	}

	return d.migrateRoomColumns()
}

func (d *DB) migrateRoomColumns() error {
	alterations := []string{
		`ALTER TABLE rooms ADD COLUMN owner_id VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE rooms ADD COLUMN expires_at TIMESTAMP WITH TIME ZONE`,
	}
	if d.dbType == DBTypeSQLite {
		alterations = []string{
			`ALTER TABLE rooms ADD COLUMN owner_id TEXT DEFAULT ''`,
			`ALTER TABLE rooms ADD COLUMN expires_at DATETIME`,
		}
	}

	for _, q := range alterations {
		if _, err := d.db.Exec(q); err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists") {
				continue
			}
			log.Printf("[DB] migrateRoomColumns note: %v", err)
		}
	}
	return nil
}

func (d *DB) CreateUser(u User) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`INSERT INTO users (id, username, email, password_hash, avatar_url, created_at) VALUES (?, ?, ?, ?, ?, ?);`)
	_, err := d.db.Exec(query, u.ID, u.Username, u.Email, u.PasswordHash, u.AvatarURL, time.Now())
	return err
}

func (d *DB) GetUserByUsernameOrEmail(username, email string) (*User, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`SELECT id, username, email, password_hash, avatar_url, created_at FROM users WHERE username = ? OR email = ? LIMIT 1;`)
	row := d.db.QueryRow(query, username, email)

	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.AvatarURL, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *DB) GetUserByID(id string) (*User, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`SELECT id, username, email, password_hash, avatar_url, created_at FROM users WHERE id = ? LIMIT 1;`)
	row := d.db.QueryRow(query, id)

	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.AvatarURL, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *DB) IsUsernameOrEmailTaken(username, email, excludeID string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`SELECT id FROM users WHERE (username = ? OR email = ?) AND id != ? LIMIT 1;`)
	var id string
	err := d.db.QueryRow(query, username, email, excludeID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *DB) UpdateUserProfile(id, username, email, avatarURL string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`UPDATE users SET username = ?, email = ?, avatar_url = ? WHERE id = ?;`)
	_, err := d.db.Exec(query, username, email, avatarURL, id)
	return err
}

func (d *DB) UpdateUserPassword(id, passwordHash string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`UPDATE users SET password_hash = ? WHERE id = ?;`)
	_, err := d.db.Exec(query, passwordHash, id)
	return err
}

func (d *DB) SaveRoom(roomID, hostID, videoID, status string, currentTime float64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var query string
	if d.dbType == DBTypePostgres {
		query = `INSERT INTO rooms (id, host_id, current_video_id, current_status, "current_time", updated_at)
			VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET
				host_id = EXCLUDED.host_id,
				current_video_id = EXCLUDED.current_video_id,
				current_status = EXCLUDED.current_status,
				"current_time" = EXCLUDED."current_time",
				updated_at = CURRENT_TIMESTAMP;`
	} else {
		query = `INSERT INTO rooms (id, host_id, current_video_id, current_status, current_time, updated_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET
				host_id = excluded.host_id,
				current_video_id = excluded.current_video_id,
				current_status = excluded.current_status,
				current_time = excluded.current_time,
				updated_at = CURRENT_TIMESTAMP;`
	}

	_, err := d.db.Exec(query, roomID, hostID, videoID, status, currentTime)
	return err
}

func (d *DB) SaveChatMessage(roomID, userID, nickname, content string, isSystem bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`INSERT INTO messages (room_id, user_id, nickname, content, is_system, created_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP);`)
	_, err := d.db.Exec(query, roomID, userID, nickname, content, isSystem)
	return err
}

func (d *DB) LoadChatHistory(roomID string, limit int) ([]ChatMessage, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`SELECT nickname, content, is_system, created_at FROM messages WHERE room_id = ? ORDER BY id DESC LIMIT ?;`)
	rows, err := d.db.Query(query, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		var isSys bool
		var createdAt time.Time

		if err := rows.Scan(&msg.Nickname, &msg.Content, &isSys, &createdAt); err != nil {
			continue
		}

		msg.Type = "chat"
		msg.IsSystem = isSys
		msg.Timestamp = createdAt.Format("15:04")
		messages = append(messages, msg)
	}

	// Reverse so clients render oldest → newest
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (d *DB) SavePlaylistItem(item PlaylistItem) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`INSERT INTO playlist_items (id, room_id, video_id, title, author, thumbnail_url, position, added_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?);`)
	_, err := d.db.Exec(query, item.ID, item.RoomID, item.VideoID, item.Title, item.Author, item.ThumbnailURL, item.Position, item.AddedBy)
	return err
}

func (d *DB) LoadPlaylist(roomID string) ([]PlaylistItem, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`SELECT id, room_id, video_id, title, author, thumbnail_url, position, added_by FROM playlist_items WHERE room_id = ? ORDER BY position ASC;`)
	rows, err := d.db.Query(query, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PlaylistItem
	for rows.Next() {
		var item PlaylistItem
		if err := rows.Scan(&item.ID, &item.RoomID, &item.VideoID, &item.Title, &item.Author, &item.ThumbnailURL, &item.Position, &item.AddedBy); err != nil {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (d *DB) DeletePlaylistItem(itemID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`DELETE FROM playlist_items WHERE id = ?;`)
	_, err := d.db.Exec(query, itemID)
	return err
}

type RoomRecord struct {
	ID             string     `db:"id"`
	Name           string     `db:"name"`
	IsPrivate      bool       `db:"is_private"`
	PasswordHash   string     `db:"password_hash"`
	OwnerID        string     `db:"owner_id"`
	ExpiresAt      *time.Time `db:"expires_at"`
	CurrentVideoID string     `db:"current_video_id"`
	CurrentStatus  string     `db:"current_status"`
	CurrentTime    float64    `db:"current_time"`
	CreatedAt      time.Time  `db:"created_at"`
}

func (d *DB) CreateRoomRecord(id, name, ownerID, pwdHash string, isPrivate bool, expiresAt *time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	isPrivInt := 0
	if isPrivate {
		isPrivInt = 1
	}
	hostID := ownerID
	if hostID == "" {
		hostID = "system"
	}

	var query string
	if d.dbType == DBTypePostgres {
		query = `INSERT INTO rooms (id, name, owner_id, host_id, password_hash, is_private, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7);`
	} else {
		query = `INSERT INTO rooms (id, name, owner_id, host_id, password_hash, is_private, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, ?);`
	}

	_, err := d.db.Exec(query, id, name, ownerID, hostID, pwdHash, isPrivInt, expiresAt)
	return err
}

func (d *DB) GetRoomByID(id string) (*RoomRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`SELECT id, name, owner_id, password_hash, is_private, expires_at, current_video_id, current_status, current_time, created_at FROM rooms WHERE id = ?;`)
	row := d.db.QueryRow(query, id)

	var rec RoomRecord
	var isPrivInt int
	err := row.Scan(&rec.ID, &rec.Name, &rec.OwnerID, &rec.PasswordHash, &isPrivInt, &rec.ExpiresAt, &rec.CurrentVideoID, &rec.CurrentStatus, &rec.CurrentTime, &rec.CreatedAt)
	if err != nil {
		return nil, err
	}
	rec.IsPrivate = isPrivInt == 1
	return &rec, nil
}

func (d *DB) ListRoomsByOwner(ownerID string) ([]RoomRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`SELECT id, name, owner_id, password_hash, is_private, expires_at, created_at FROM rooms WHERE owner_id = ? AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP) ORDER BY created_at DESC;`)
	rows, err := d.db.Query(query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RoomRecord
	for rows.Next() {
		var rec RoomRecord
		var isPrivInt int
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.OwnerID, &rec.PasswordHash, &isPrivInt, &rec.ExpiresAt, &rec.CreatedAt); err != nil {
			continue
		}
		rec.IsPrivate = isPrivInt == 1
		list = append(list, rec)
	}
	return list, nil
}

func (d *DB) DeleteOwnedRoom(id, ownerID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := d.Rebind(`DELETE FROM rooms WHERE id = ? AND owner_id = ?;`)
	res, err := d.db.Exec(query, id, ownerID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("room not found or unauthorized")
	}
	return nil
}
