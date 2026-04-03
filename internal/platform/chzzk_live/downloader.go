package chzzk_live

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const retryGracePeriod = 60 * time.Second
const retryDelay = 5 * time.Second
const progressPollInterval = 2 * time.Second

func RecordLive(session *Session, hlsURL string, outputPath string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		slog.Warn("ffmpeg binary not found")
		return errors.New("ffmpeg not found in PATH")
	}

	deadline := time.Now().Add(retryGracePeriod)
	attempt := 0
	partFiles := []string{}

	for {
		attempt++
		partPath := partOutputPath(outputPath, attempt)
		partFiles = append(partFiles, partPath)
		session.UpdateStatus("downloading")
		slog.Info("starting live recording attempt", "attempt", attempt, "output", partPath)

		cmd := exec.CommandContext(session.Context(), "ffmpeg",
			"-hide_banner",
			"-loglevel", "warning",
			"-user_agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Mobile/15E148 Safari/604.1",
			"-i", hlsURL,
			"-c", "copy",
			"-movflags", "+faststart",
			"-y",
			partPath,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		stopProgress := make(chan struct{})
		go trackProgress(session, stopProgress, partFiles)
		err := cmd.Run()
		close(stopProgress)
		updateProgressFromFiles(session, partFiles)

		if err == nil {
			slog.Info("live recording finished", "output", partPath)
			if len(partFiles) == 1 {
				if partFiles[0] != outputPath {
					if renameErr := os.Rename(partFiles[0], outputPath); renameErr != nil {
						return fmt.Errorf("rename final live output: %w", renameErr)
					}
				}
				return nil
			}
			return concatParts(outputPath, partFiles)
		}
		if session.Context().Err() != nil {
			return session.Context().Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("live recording failed after retry grace period: %w", err)
		}
		slog.Warn("live recording attempt failed, retrying", "attempt", attempt, "error", err, "retry_in", retryDelay.String(), "part", partPath)
		time.Sleep(retryDelay)
	}
}

func trackProgress(session *Session, stop <-chan struct{}, files []string) {
	ticker := time.NewTicker(progressPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-session.Context().Done():
			return
		case <-ticker.C:
			updateProgressFromFiles(session, files)
		}
	}
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

func partOutputPath(outputPath string, attempt int) string {
	ext := filepath.Ext(outputPath)
	base := outputPath[:len(outputPath)-len(ext)]
	return fmt.Sprintf("%s.part%d%s", base, attempt, ext)
}

func concatParts(outputPath string, parts []string) error {
	listFile, err := os.CreateTemp(filepath.Dir(outputPath), "chzzk-live-concat-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(listFile.Name())
	defer listFile.Close()

	for _, part := range parts {
		if _, err := fmt.Fprintf(listFile, "file '%s'\n", escapeFFmpegConcatPath(part)); err != nil {
			return err
		}
	}
	if err := listFile.Close(); err != nil {
		return err
	}

	cmd := exec.Command("ffmpeg",
		"-hide_banner",
		"-loglevel", "warning",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile.Name(),
		"-c", "copy",
		"-y",
		outputPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("concat live parts: %w", err)
	}
	return nil
}

func escapeFFmpegConcatPath(path string) string {
	return filepath.ToSlash(path)
}
