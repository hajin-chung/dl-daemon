package chzzk

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	"golang.org/x/sync/semaphore"
)

type playlist struct {
	Init     string
	Segments []string
}

func DownloadHLSVideo(session *Session, client *Client, videoURL string, outputName string) error {
	if !HasBinary("ffmpeg") {
		slog.Warn("ffmpeg binary not found")
		return errors.New("ffmpeg not found in PATH")
	}

	playlistURL, err := getPlaylistURL(client, videoURL)
	if err != nil {
		return err
	}

	playlistHLS, err := client.GetBody(playlistURL)
	if err != nil {
		return err
	}

	playlist, err := parsePlaylistHLS(playlistURL, playlistHLS)
	if err != nil {
		return err
	}

	totalSegments := int64(len(playlist.Segments))
	slog.Info("parsed hls playlist", "segments", totalSegments, "output", outputName)
	session.SetBytes(0, totalSegments)

	cmd := exec.CommandContext(session.Context(), "ffmpeg",
		"-hide_banner",
		"-i", "pipe:0",
		"-c", "copy",
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof+global_sidx",
		"-y",
		outputName,
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
		nextExpected := -1
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

	go func() {
		res, err := client.GetWithRetry(playlist.Init, 10)
		if err != nil {
			resultCh <- segmentResult{err: err}
			return
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			resultCh <- segmentResult{err: err}
			return
		}
		resultCh <- segmentResult{index: -1, data: data}
	}()

	for i, url := range playlist.Segments {
		if err := sem.Acquire(ctx, 1); err != nil {
			_ = cmd.Process.Kill()
			return err
		}
		go func(i int, url string) {
			defer sem.Release(1)
			res, err := client.GetWithRetry(url, 10)
			if err != nil {
				resultCh <- segmentResult{err: err}
				return
			}
			defer res.Body.Close()
			data, err := io.ReadAll(res.Body)
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
		slog.Error("hls download failed", "error", err)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}

	err = cmd.Wait()
	if err == nil {
		slog.Info("hls download complete", "output", outputName)
	}
	return err
}

func getPlaylistURL(client *Client, url string) (string, error) {
	masterHLS, err := client.GetBody(url)
	if err != nil {
		return "", err
	}

	bandwidthRe := regexp.MustCompile(`BANDWIDTH=(\d+),`)
	lines := strings.Split(masterHLS, "\n")
	maxBandwidth := 0
	playlistPath := ""
	for i := 0; i < len(lines); i++ {
		match := bandwidthRe.FindStringSubmatch(lines[i])
		if len(match) < 2 {
			continue
		}
		bandwidth, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if bandwidth > maxBandwidth && i+1 < len(lines) {
			maxBandwidth = bandwidth
			playlistPath = strings.TrimSpace(lines[i+1])
		}
	}
	if playlistPath == "" {
		return "", errors.New("no playlist url found")
	}
	return UrlJoin(url, playlistPath)
}

func parsePlaylistHLS(url string, hls string) (*playlist, error) {
	initRe := regexp.MustCompile(`#EXT-X-MAP:URI="(.+)"`)
	match := initRe.FindStringSubmatch(hls)
	if len(match) < 2 {
		return nil, errors.New("no init uri found")
	}
	initURL, err := UrlJoin(url, match[1])
	if err != nil {
		return nil, err
	}

	segments := []string{}
	segmentRe := regexp.MustCompile(`#EXTINF:`)
	lines := strings.Split(hls, "\n")
	for i := 0; i < len(lines); i++ {
		if !segmentRe.MatchString(lines[i]) {
			continue
		}
		if i+1 >= len(lines) {
			return nil, errors.New("malformed playlist: missing segment url")
		}
		segmentURL, err := UrlJoin(url, strings.TrimSpace(lines[i+1]))
		if err != nil {
			return nil, err
		}
		segments = append(segments, segmentURL)
		i++
	}

	return &playlist{Init: initURL, Segments: segments}, nil
}
