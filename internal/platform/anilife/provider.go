package anilife

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"deps.me/dl-daemon/internal/model"
	"deps.me/dl-daemon/internal/platform"
)

type Provider struct {
	client    *Client
	outputDir string
}

func NewProvider(outputDir string) *Provider {
	if outputDir == "" {
		outputDir = "."
	}
	return &Provider{client: NewClient(), outputDir: outputDir}
}

func (p *Provider) Name() string {
	return "anilife"
}

func (p *Provider) Watch(id string) ([]model.Content, error) {
	slog.Debug("fetching anilife episode list", "platform", p.Name(), "target_id", id)
	_, episodes, err := p.client.GetAnime(id)
	if err != nil {
		return nil, err
	}

	items := make([]model.Content, 0, len(episodes))
	for _, episode := range episodes {
		items = append(items, model.Content{
			VideoId:  episode.URL,
			TargetId: id,
			Platform: p.Name(),
			Title:    fmt.Sprintf("%s %s", episode.Num, episode.Title),
		})
	}
	return items, nil
}

func (p *Provider) Download(content model.Content) (platform.DownloadSession, error) {
	slog.Info("preparing anilife download", "video_id", content.VideoId, "target_id", content.TargetId, "title", content.Title)

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
	anime, episodes, err := p.client.GetAnime(content.TargetId)
	if err != nil {
		return err
	}

	var episode *Episode
	for _, candidate := range episodes {
		if candidate.URL == content.VideoId {
			episode = candidate
			break
		}
	}
	if episode == nil {
		return fmt.Errorf("episode not found for content %q", content.VideoId)
	}

	hlsURL, err := p.client.GetEpisodeHLS(episode, anime)
	if err != nil {
		return err
	}

	outputPath := filepath.Join(p.outputDir, outputName(anime.Title, episode))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	return DownloadHLSVideo(session, hlsURL, outputPath)
}
