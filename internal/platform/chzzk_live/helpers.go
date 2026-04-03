package chzzk_live

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

func sanitizeFilename(filename string) string {
	re := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	filename = re.ReplaceAllString(filename, "_")
	filename = strings.TrimSpace(filename)
	filename = strings.TrimRight(filename, ".")
	return filename
}

func outputName(channelName string, liveTitle string, liveID int, openDate string) string {
	date := openDate
	if ts, err := time.Parse("2006-01-02 15:04:05", openDate); err == nil {
		date = ts.Format("2006-01-02 15-04-05")
	}
	return sanitizeFilename(fmt.Sprintf("%s %s - %s [%d].mp4", date, channelName, liveTitle, liveID))
}
