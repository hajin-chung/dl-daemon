package db

import (
	"testing"

	"deps.me/dl-daemon/internal/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()

	conn, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite memory db: %v", err)
	}
	if err := initSchema(conn); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return &DB{conn: conn}
}

func TestSetStatusUpdatesRow(t *testing.T) {
	db := newTestDB(t)

	item := model.Content{VideoId: "vid-1", Platform: "chzzk", Title: "test"}
	if err := db.InsertDownload(item); err != nil {
		t.Fatalf("insert download: %v", err)
	}

	if err := db.SetStatus(item, "failed", "boom"); err != nil {
		t.Fatalf("set status: %v", err)
	}

	var row struct {
		Status   string `db:"status"`
		ErrorMsg string `db:"error_msg"`
	}
	if err := db.conn.Get(&row, `SELECT status, error_msg FROM downloads WHERE video_id = ?`, item.DownloadID()); err != nil {
		t.Fatalf("query row: %v", err)
	}

	if row.Status != "failed" {
		t.Fatalf("status = %q, want failed", row.Status)
	}
	if row.ErrorMsg != "boom" {
		t.Fatalf("error_msg = %q, want boom", row.ErrorMsg)
	}
}

func TestUpdateProgressUpdatesFields(t *testing.T) {
	db := newTestDB(t)

	item := model.Content{VideoId: "vid-2", Platform: "anilife", Title: "episode"}
	if err := db.InsertDownload(item); err != nil {
		t.Fatalf("insert download: %v", err)
	}

	progress := model.DownloadProgress{
		VideoId:      item.VideoId,
		Platform:     item.Platform,
		BytesWritten: 123,
		TotalBytes:   456,
		Status:       "downloading",
	}
	if err := db.UpdateProgress(progress); err != nil {
		t.Fatalf("update progress: %v", err)
	}

	var row struct {
		BytesWritten int64  `db:"bytes_written"`
		TotalBytes   int64  `db:"total_bytes"`
		Status       string `db:"status"`
	}
	if err := db.conn.Get(&row, `SELECT bytes_written, total_bytes, status FROM downloads WHERE video_id = ?`, item.DownloadID()); err != nil {
		t.Fatalf("query row: %v", err)
	}

	if row.BytesWritten != 123 || row.TotalBytes != 456 || row.Status != "downloading" {
		t.Fatalf("unexpected row: %+v", row)
	}
}

func TestInsertDownloadNamespacesIDsByPlatform(t *testing.T) {
	db := newTestDB(t)

	chzzk := model.Content{VideoId: "shared-id", Platform: "chzzk", Title: "vod"}
	anilife := model.Content{VideoId: "shared-id", Platform: "anilife", Title: "episode"}

	if err := db.InsertDownload(chzzk); err != nil {
		t.Fatalf("insert chzzk: %v", err)
	}
	if err := db.InsertDownload(anilife); err != nil {
		t.Fatalf("insert anilife: %v", err)
	}

	var count int
	if err := db.conn.Get(&count, `SELECT COUNT(*) FROM downloads`); err != nil {
		t.Fatalf("count downloads: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}
