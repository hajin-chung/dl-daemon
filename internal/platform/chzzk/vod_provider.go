package chzzk

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"deps.me/dl-daemon/internal/model"
	"deps.me/dl-daemon/internal/platform"
)

type VODProvider struct {
	client    *Client
	outputDir string
}

func NewVODProvider(outputDir string, aut string, ses string) *VODProvider {
	if outputDir == "" {
		outputDir = "."
	}
	return &VODProvider{
		client:    NewClient(aut, ses),
		outputDir: outputDir,
	}
}

func (p *VODProvider) Name() string {
	return "chzzk"
}

func (p *VODProvider) Watch(id string) ([]model.Content, error) {
	videos, err := p.client.GetVideoList(id)
	if err != nil {
		return nil, err
	}

	items := make([]model.Content, 0, len(videos))
	for _, video := range videos {
		items = append(items, model.Content{
			VideoId:  strconv.Itoa(video.VideoNo),
			TargetId: id,
			Platform: p.Name(),
			Title:    video.Title,
		})
	}
	return items, nil
}

func (p *VODProvider) Download(content model.Content) (platform.DownloadSession, error) {
	videoNo, err := strconv.Atoi(content.VideoId)
	if err != nil {
		return nil, fmt.Errorf("invalid video id %q: %w", content.VideoId, err)
	}

	session := NewSession(content.VideoId, p.Name())
	session.UpdateStatus("starting")

	go func() {
		err := p.download(session, videoNo)
		if err != nil {
			session.UpdateStatus("failed")
			session.Finish(err)
			return
		}
		session.UpdateStatus("complete")
		session.Finish(nil)
	}()

	return session, nil
}

func (p *VODProvider) download(session *Session, videoNo int) error {
	info, err := p.client.GetVideoInfo(videoNo)
	if err != nil {
		return err
	}

	outputName, err := OutputName(info)
	if err != nil {
		return err
	}
	outputPath := filepath.Join(p.outputDir, outputName)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	videoURL, err := p.client.GetVideoURL(videoNo)
	if err != nil {
		return err
	}

	slog.Info("chzzk download starting", "video_no", videoNo, "type", videoURL.Type, "output", outputPath)
	session.UpdateStatus("downloading")

	switch videoURL.Type {
	case HLS:
		return DownloadHLSVideo(session, p.client, videoURL.URL, outputPath)
	case DASH:
		return DownloadDASHVideo(session, videoURL.URL, outputPath)
	default:
		return fmt.Errorf("unsupported video type: %s", videoURL.Type)
	}
}
