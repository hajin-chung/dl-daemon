package chzzk_live

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

const retryGracePeriod = 60 * time.Second
const retryDelay = 5 * time.Second

func RecordLive(session *Session, hlsURL string, outputPath string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		slog.Warn("ffmpeg binary not found")
		return errors.New("ffmpeg not found in PATH")
	}

	deadline := time.Now().Add(retryGracePeriod)
	attempt := 0
	for {
		attempt++
		session.UpdateStatus("downloading")
		slog.Info("starting live recording attempt", "attempt", attempt, "output", outputPath)

		cmd := exec.CommandContext(session.Context(), "ffmpeg",
			"-hide_banner",
			"-loglevel", "warning",
			"-user_agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Mobile/15E148 Safari/604.1",
			"-i", hlsURL,
			"-c", "copy",
			"-movflags", "+faststart",
			"-y",
			outputPath,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err == nil {
			slog.Info("live recording finished", "output", outputPath)
			return nil
		}
		if session.Context().Err() != nil {
			return session.Context().Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("live recording failed after retry grace period: %w", err)
		}
		slog.Warn("live recording attempt failed, retrying", "attempt", attempt, "error", err, "retry_in", retryDelay.String())
		time.Sleep(retryDelay)
	}
}
