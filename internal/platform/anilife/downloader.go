package anilife

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"

	"golang.org/x/sync/semaphore"
)

type playlist struct {
	Segments []string
}

func DownloadHLSVideo(session *Session, client *Client, videoURL string, outputPath string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		slog.Warn("ffmpeg binary not found")
		return errors.New("ffmpeg not found in PATH")
	}

	hlsBody, err := client.GetBodyWithReferer(videoURL, baseURL)
	if err != nil {
		return err
	}

	playlist, err := parsePlaylistHLS(videoURL, hlsBody)
	if err != nil {
		return err
	}

	totalSegments := int64(len(playlist.Segments))
	session.SetBytes(0, totalSegments)
	slog.Info("parsed anilife hls playlist", "segments", totalSegments, "output", outputPath)

	cmd := exec.CommandContext(session.Context(), "ffmpeg",
		"-hide_banner",
		"-i", "pipe:0",
		"-c", "copy",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)

	ffmpegStdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	ctx := session.Context()
	maxWorkers := int64(runtime.GOMAXPROCS(0) * 2)
	sem := semaphore.NewWeighted(maxWorkers)
	completedSegments := atomic.Int64{}

	type segmentResult struct {
		index int
		data  []byte
		err   error
	}
	resultCh := make(chan segmentResult, maxWorkers)
	doneCh := make(chan error, 1)

	go func() {
		nextExpected := 0
		buffer := make(map[int][]byte)

		for {
			select {
			case <-ctx.Done():
				doneCh <- ctx.Err()
				return
			case res := <-resultCh:
				if res.err != nil {
					doneCh <- res.err
					return
				}
				buffer[res.index] = res.data

				for {
					data, ok := buffer[nextExpected]
					if !ok {
						break
					}
					if _, err := ffmpegStdin.Write(data); err != nil {
						doneCh <- fmt.Errorf("ffmpeg write error: %w", err)
						return
					}
					delete(buffer, nextExpected)
					nextExpected++
					if nextExpected == len(playlist.Segments) {
						_ = ffmpegStdin.Close()
						doneCh <- nil
						return
					}
				}
			}
		}
	}()

	for i, url := range playlist.Segments {
		if err := sem.Acquire(ctx, 1); err != nil {
			_ = cmd.Process.Kill()
			return err
		}
		go func(i int, url string) {
			defer sem.Release(1)
			data, err := client.GetBytesWithHeaders(url, map[string]string{
				"Referer": baseURL,
				"Origin":  baseURL,
			})
			if err != nil {
				resultCh <- segmentResult{err: err}
				return
			}
			resultCh <- segmentResult{index: i, data: data}
			written := completedSegments.Add(1)
			session.SetBytes(written, totalSegments)
			session.UpdateStatus("downloading")
		}(i, url)
	}

	if err := <-doneCh; err != nil {
		slog.Error("anilife hls download failed", "error", err)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}

	err = cmd.Wait()
	if err == nil {
		slog.Info("anilife hls download complete", "output", outputPath)
	}
	return err
}

func parsePlaylistHLS(base string, hls string) (*playlist, error) {
	segments := []string{}
	lines := strings.Split(hls, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "#EXTINF") {
			continue
		}
		if i+1 >= len(lines) {
			return nil, errors.New("malformed playlist: missing segment url")
		}
		segmentURL := strings.TrimSpace(lines[i+1])
		segmentURL = absoluteURL(base, segmentURL)
		segments = append(segments, segmentURL)
		i++
	}
	if len(segments) == 0 {
		return nil, errors.New("no segments found in playlist")
	}
	return &playlist{Segments: segments}, nil
}
