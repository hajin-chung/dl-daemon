package anilife

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

func DownloadHLSVideo(session *Session, videoURL string, outputPath string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		slog.Warn("ffmpeg binary not found")
		return errors.New("ffmpeg not found in PATH")
	}

	cmd := exec.CommandContext(session.Context(), "ffmpeg",
		"-hide_banner",
		"-loglevel", "warning",
		"-user_agent", userAgent,
		"-headers", fmt.Sprintf("Referer: %s\r\nOrigin: %s\r\n", baseURL, baseURL),
		"-i", videoURL,
		"-c", "copy",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	session.UpdateStatus("downloading")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
