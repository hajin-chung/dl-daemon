package chzzk_live

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const retryGracePeriod = 60 * time.Second
const playlistPollInterval = 2 * time.Second
const streamEndGracePeriod = 30 * time.Second

func RecordLive(session *Session, client *Client, variantURL string, outputPath string, liveID int) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		slog.Warn("ffmpeg binary not found")
		return errors.New("ffmpeg not found in PATH")
	}

	work := workDir(outputPath, liveID)
	segmentsDir := filepath.Join(work, "segments")
	if err := os.MkdirAll(segmentsDir, 0755); err != nil {
		return err
	}

	initPath := filepath.Join(work, "init.m4s")
	seen := map[string]bool{}
	segmentFiles := []string{}
	lastNewSegmentAt := time.Now()
	initDownloaded := false

	slog.Info("starting live segment capture", "variant_url", variantURL, "work_dir", work)
	session.UpdateStatus("downloading")

	for {
		playlistBody, err := client.GetBody(variantURL)
		if err != nil {
			if time.Since(lastNewSegmentAt) > retryGracePeriod {
				return fmt.Errorf("playlist fetch failed after grace period: %w", err)
			}
			slog.Warn("playlist fetch failed, retrying", "error", err)
			time.Sleep(playlistPollInterval)
			continue
		}

		playlist, err := parseMediaPlaylist(variantURL, playlistBody)
		if err != nil {
			return err
		}

		if !initDownloaded {
			if err := downloadFile(client, playlist.InitURL, initPath); err != nil {
				return fmt.Errorf("download init segment: %w", err)
			}
			initDownloaded = true
		}

		newSegments := 0
		for _, segmentURL := range playlist.Segments {
			if seen[segmentURL] {
				continue
			}
			seen[segmentURL] = true
			localPath := localSegmentPath(work, len(segmentFiles)+1, segmentExt(segmentURL))
			if err := downloadFile(client, segmentURL, localPath); err != nil {
				return fmt.Errorf("download segment: %w", err)
			}
			segmentFiles = append(segmentFiles, localPath)
			newSegments++
		}

		updateProgressFromFiles(session, append([]string{initPath}, segmentFiles...))
		if newSegments > 0 {
			lastNewSegmentAt = time.Now()
			slog.Debug("downloaded live segments", "count", newSegments, "total_segments", len(segmentFiles))
		}

		if playlist.Ended {
			slog.Info("playlist ended, remuxing live capture")
			break
		}
		if time.Since(lastNewSegmentAt) > streamEndGracePeriod {
			slog.Info("no new live segments within grace period, ending capture", "grace_period", streamEndGracePeriod.String())
			break
		}
		select {
		case <-session.Context().Done():
			return session.Context().Err()
		case <-time.After(playlistPollInterval):
		}
	}

	if len(segmentFiles) == 0 {
		return errors.New("no live segments downloaded")
	}
	return remuxFragments(outputPath, initPath, segmentFiles)
}

func downloadFile(client *Client, url string, path string) error {
	res, err := client.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, res.Body)
	return err
}

func updateProgressFromFiles(session *Session, files []string) {
	var total int64
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		total += info.Size()
	}
	session.SetBytes(total, 0)
}

func remuxFragments(outputPath string, initPath string, segments []string) error {
	cmd := exec.Command("ffmpeg",
		"-hide_banner",
		"-loglevel", "warning",
		"-i", "pipe:0",
		"-c", "copy",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	writeFile := func(path string) error {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(stdin, f)
		return err
	}

	if err := writeFile(initPath); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return err
	}
	for _, path := range segments {
		if err := writeFile(path); err != nil {
			_ = stdin.Close()
			_ = cmd.Wait()
			return err
		}
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func segmentExt(url string) string {
	trimmed := url
	if idx := strings.Index(trimmed, "?"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	ext := filepath.Ext(trimmed)
	if ext == "" {
		return ".seg"
	}
	return ext
}
