package chzzk_live

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"deps.me/dl-daemon/internal/model"
	"deps.me/dl-daemon/internal/platform"
)

type Provider struct {
	client    *Client
	outputDir string
}

func NewProvider(outputDir string, aut string, ses string) *Provider {
	if outputDir == "" {
		outputDir = "."
	}
	return &Provider{client: NewClient(aut, ses), outputDir: outputDir}
}

func (p *Provider) Name() string { return "chzzk_live" }

func (p *Provider) Watch(id string) ([]model.Content, error) {
	status, err := p.client.GetLiveStatus(id)
	if err != nil {
		return nil, err
	}
	if status.Status != "OPEN" {
		return nil, nil
	}
	detail, err := p.client.GetLiveDetail(id)
	if err != nil {
		return nil, err
	}
	item := model.Content{
		VideoId:  strconv.Itoa(detail.LiveID),
		TargetId: id,
		Platform: p.Name(),
		Title:    detail.LiveTitle,
	}
	return []model.Content{item}, nil
}

func (p *Provider) Download(content model.Content) (platform.DownloadSession, error) {
	slog.Info("preparing chzzk live recording", "live_id", content.VideoId, "title", content.Title)
	session := NewSession(content.VideoId, p.Name())
	session.UpdateStatus("starting")
	go func() {
		err := p.download(session, content)
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

func (p *Provider) download(session *Session, content model.Content) error {
	detail, err := p.client.GetLiveDetail(content.TargetId)
	if err != nil {
		return err
	}
	if strconv.Itoa(detail.LiveID) != content.VideoId {
		return fmt.Errorf("live session changed before download start: expected %s, got %d", content.VideoId, detail.LiveID)
	}
	hlsURL, err := detail.HLSPath(p.client)
	if err != nil {
		return err
	}
	outputPath := filepath.Join(p.outputDir, outputName(detail.Channel.ChannelName, detail.LiveTitle, detail.LiveID, detail.OpenDate))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	return RecordLive(session, hlsURL, outputPath)
}
