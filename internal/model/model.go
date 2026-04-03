package model

import "fmt"

type Target struct {
	Platform  string `db:"platform"`
	Id        string `db:"id"`
	Label     string `db:"label"`
	OutputDir string `db:"output_dir"`
}

type Content struct {
	VideoId   string
	TargetId  string
	Platform  string
	Title     string
	OutputDir string
}

func (c Content) DownloadID() string {
	return DownloadID(c.Platform, c.VideoId)
}

type DownloadProgress struct {
	VideoId      string `json:"video_id"`
	Platform     string `json:"platform"`
	BytesWritten int64  `json:"bytes_written"`
	TotalBytes   int64  `json:"total_bytes"`
	Status       string `json:"status"`
}

func (p DownloadProgress) DownloadID() string {
	return DownloadID(p.Platform, p.VideoId)
}

func DownloadID(platform string, videoID string) string {
	if platform == "" {
		return videoID
	}
	return fmt.Sprintf("%s:%s", platform, videoID)
}

