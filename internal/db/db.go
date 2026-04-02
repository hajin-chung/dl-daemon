package db

import (
	"fmt"
	"os"
	"path/filepath"

	"deps.me/dl-daemon/internal/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sqlx.DB
}

type DownloadRow struct {
	VideoID      string `db:"video_id"`
	Title        string `db:"title"`
	Platform     string `db:"platform"`
	Status       string `db:"status"`
	BytesWritten int64  `db:"bytes_written"`
	TotalBytes   int64  `db:"total_bytes"`
	ErrorMsg     string `db:"error_msg"`
}

func OpenDatabase() (*DB, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("Could not find config dir: %w", err)
	}

	appPath := filepath.Join(baseDir, "dld")
	dbPath := filepath.Join(appPath, "dld.db")

	err = os.MkdirAll(appPath, 0755)
	if err != nil {
		return nil, fmt.Errorf("Could not create directory: %w", err)
	}

	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}

	err = initSchema(db)
	if err != nil {
		return nil, err
	}

	return &DB{conn: db}, nil
}

func initSchema(db *sqlx.DB) error {
	const query = `
	CREATE TABLE IF NOT EXISTS metadata (
		key TEXT PRIMARY KEY,
		value TEXT
	);
	CREATE TABLE IF NOT EXISTS targets (
		target_id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT,
		id TEXT,
		label TEXT
	);
	CREATE TABLE IF NOT EXISTS downloads (
		video_id TEXT PRIMARY KEY,
		title TEXT,
		platform TEXT,
		status TEXT,
		bytes_written INTEGER DEFAULT 0,
		total_bytes INTEGER DEFAULT 0,
		error_msg TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(query)
	return err
}

func (db *DB) GetTargets() ([]model.Target, error) {
	targets := []model.Target{}
	query := `SELECT platform, id, label FROM targets ORDER BY target_id ASC`
	err := db.conn.Select(&targets, query)
	return targets, err
}

func (db *DB) SetMetadata(key string, value string) error {
	query := `INSERT OR REPLACE INTO metadata(key, value) VALUES (?, ?);`
	_, err := db.conn.Exec(query, key, value)
	return err
}

func (db *DB) GetMetadata(key string) (string, error) {
	var value string
	query := `SELECT value FROM metadata WHERE key = ?;`
	err := db.conn.Get(&value, query, key)
	return value, err
}

type MetadataRow struct {
	Key   string `db:"key"`
	Value string `db:"value"`
}

func (db *DB) ListMetadata() ([]MetadataRow, error) {
	rows := []MetadataRow{}
	query := `SELECT key, value FROM metadata ORDER BY key ASC`
	err := db.conn.Select(&rows, query)
	return rows, err
}

func (db *DB) AddTarget(target model.Target) error {
	query := `
	INSERT INTO targets(platform, id, label)
	VALUES (?, ?, ?);
	`
	_, err := db.conn.Exec(query, target.Platform, target.Id, target.Label)
	return err
}

func (db *DB) RemoveTarget(platform string, id string) error {
	query := `DELETE FROM targets WHERE platform = ? AND id = ?;`
	_, err := db.conn.Exec(query, platform, id)
	return err
}

func (db *DB) Exists(item model.Content) bool {
	exists := false
	query := "SELECT EXISTS(SELECT 1 FROM downloads WHERE video_id = ?)"
	_ = db.conn.Get(&exists, query, item.DownloadID())
	return exists
}

func (db *DB) InsertDownload(item model.Content) error {
	query := `
	INSERT INTO downloads(video_id, title, platform, status)
	VALUES (?, ?, ?, ?);
	`
	_, err := db.conn.Exec(query, item.DownloadID(), item.Title, item.Platform, "pending")
	return err
}

func (db *DB) UpdateProgress(progress model.DownloadProgress) error {
	query := `
	UPDATE downloads
	SET bytes_written = ?, total_bytes = ?, status = ?
	WHERE video_id = ?;
	`
	_, err := db.conn.Exec(query, progress.BytesWritten, progress.TotalBytes, progress.Status, progress.DownloadID())
	return err
}

func (db *DB) SetStatus(item model.Content, status string, errorMsg string) error {
	query := `
	UPDATE downloads
	SET status = ?, error_msg = ?
	WHERE video_id = ?;
	`
	_, err := db.conn.Exec(query, status, errorMsg, item.DownloadID())
	return err
}

func (db *DB) ListDownloads() ([]DownloadRow, error) {
	rows := []DownloadRow{}
	query := `
	SELECT video_id, title, platform, status, bytes_written, total_bytes, error_msg
	FROM downloads
	ORDER BY updated_at DESC, video_id ASC
	`
	err := db.conn.Select(&rows, query)
	return rows, err
}

