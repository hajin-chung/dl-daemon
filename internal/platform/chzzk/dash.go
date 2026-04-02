package chzzk

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
)

func DownloadDASHVideo(session *Session, videoURL string, outputName string) error {
	if !HasBinary("axel") {
		slog.Warn("axel binary not found")
		return errors.New("axel not found in PATH")
	}

	slog.Info("starting dash download", "output", outputName)

	cmd := exec.CommandContext(session.Context(), "axel", "-n", "8", "-o", outputName, videoURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		slog.Error("dash download failed", "error", err)
		return err
	}

	session.UpdateStatus("downloading")
	slog.Info("dash download complete", "output", outputName)
	return nil
}
