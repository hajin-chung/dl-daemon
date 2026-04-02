package chzzk

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

func FormatDate(rawDate string) (string, error) {
	date, err := time.Parse("2006-01-02 15:04:05", rawDate)
	if err != nil {
		return "", err
	}
	return date.Format("06.01.02"), nil
}

func SanitizeFileName(name string) string {
	sanitized := name
	for _, char := range strings.Split(`\\/:*?"<>|`, "") {
		sanitized = strings.ReplaceAll(sanitized, char, "")
	}
	return sanitized
}

func OutputName(info *VideoData) (string, error) {
	date, err := FormatDate(info.Date)
	if err != nil {
		return "", err
	}
	return SanitizeFileName(fmt.Sprintf("%s %s [%d].mp4", date, info.Title, info.VideoNo)), nil
}

func UrlJoin(base string, part string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	partURL, err := url.Parse(part)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(partURL).String(), nil
}

func HasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
