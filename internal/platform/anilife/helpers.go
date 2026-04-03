package anilife

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func sanitizeFilename(filename string) string {
	re := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	filename = re.ReplaceAllString(filename, "_")
	filename = strings.TrimSpace(filename)
	filename = strings.TrimRight(filename, ".")
	return filename
}

func outputName(animeTitle string, episode *Episode) string {
	title := strings.TrimSpace(fmt.Sprintf("%s.%s.mp4", leftPadString(episode.Num, 3, "0"), episode.Title))
	return filepath.Join(sanitizeFilename(animeTitle), sanitizeFilename(title))
}

func leftPadString(str string, width int, pad string) string {
	if len(str) >= width {
		return str
	}
	return strings.Repeat(pad, width-len(str)) + str
}
