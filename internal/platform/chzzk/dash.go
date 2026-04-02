package chzzk

import (
	"errors"
	"os"
	"os/exec"
)

func DownloadDASHVideo(session *Session, videoURL string, outputName string) error {
	if !HasBinary("axel") {
		return errors.New("axel not found in PATH")
	}

	cmd := exec.CommandContext(session.Context(), "axel", "-n", "8", "-o", outputName, videoURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	session.UpdateStatus("downloading")
	return nil
}
