// TODO: add logging messages
// TODO: let time intervals be configurable

package manager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"deps.me/dl-daemon/internal/db"
	"deps.me/dl-daemon/internal/model"
	"deps.me/dl-daemon/internal/platform"
	"deps.me/dl-daemon/internal/platform/chzzk"
)

type Manager struct {
	providers      map[string]platform.Provider
	db             *db.DB
	mu             sync.RWMutex
	activeSessions map[string]platform.DownloadSession
}

func New(db *db.DB) *Manager {
	providers := map[string]platform.Provider{}

	baseOutputDir := "."
	if home, err := os.UserHomeDir(); err == nil {
		baseOutputDir = filepath.Join(home, "Downloads", "dld", "chzzk")
	}
	providers["chzzk"] = chzzk.NewVODProvider(baseOutputDir, "", "")
	for name := range providers {
		slog.Info("provider registered", "provider", name)
	}

	manager := Manager{
		providers:      providers,
		db:             db,
		mu:             sync.RWMutex{},
		activeSessions: map[string]platform.DownloadSession{},
	}

	return &manager
}

func (m *Manager) Start() {
	_ = m.Run(context.Background())
}

func (m *Manager) Run(ctx context.Context) error {
	slog.Info("manager starting")
	watchErrCh := make(chan error, 1)
	syncErrCh := make(chan error, 1)

	go func() {
		watchErrCh <- m.watchLoop(ctx)
	}()
	go func() {
		syncErrCh <- m.syncLoop(ctx)
	}()

	select {
	case <-ctx.Done():
		slog.Info("manager context canceled, stopping active sessions")
		m.stopActiveSessions()
		return nil
	case err := <-watchErrCh:
		slog.Warn("watch loop exited", "error", err)
		m.stopActiveSessions()
		if err == nil || err == context.Canceled {
			return nil
		}
		return fmt.Errorf("watch loop: %w", err)
	case err := <-syncErrCh:
		slog.Warn("sync loop exited", "error", err)
		m.stopActiveSessions()
		if err == nil || err == context.Canceled {
			return nil
		}
		return fmt.Errorf("sync loop: %w", err)
	}
}

func (m *Manager) watchLoop(ctx context.Context) error {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	slog.Info("watch loop started", "interval", "20s")

	for {
		select {
		case <-ctx.Done():
			slog.Debug("watch loop context done")
			return ctx.Err()
		case <-ticker.C:
			logs := slog.With("loop", "watch")
			logs.Debug("tick")
			targets, err := m.db.GetTargets()
			if err != nil {
				logs.Error("failed to fetch targets", "error", err)
				return err
			}
			logs.Debug("targets fetched", "count", len(targets))
			for _, t := range targets {
				providerLog := logs.With("platform", t.Platform, "target_id", t.Id, "label", t.Label)
				p, ok := m.providers[t.Platform]
				if !ok {
					providerLog.Warn("provider not registered")
					continue
				}
				items, err := p.Watch(t.Id)
				if err != nil {
					providerLog.Warn("provider watch failed", "error", err)
					continue
				}
				providerLog.Info("watch completed", "items_found", len(items))

				for _, item := range items {
					itemLog := providerLog.With("video_id", item.VideoId, "title", item.Title)
					if m.db.Exists(item) {
						itemLog.Debug("download already recorded, skipping")
						continue
					}
					itemLog.Info("new content discovered")
					if err := m.db.InsertDownload(item); err != nil {
						itemLog.Warn("failed to insert download", "error", err)
						continue
					}
					if err := m.startDownload(item); err != nil {
						itemLog.Error("failed to start download", "error", err)
						_ = m.db.SetStatus(item, "failed", err.Error())
					}
				}
			}
		}
	}
}

func (m *Manager) syncLoop(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	slog.Info("sync loop started", "interval", "5s")

	for {
		select {
		case <-ctx.Done():
			slog.Debug("sync loop context done")
			return ctx.Err()
		case <-ticker.C:
			logs := slog.With("loop", "sync")
			logs.Debug("tick")
			m.mu.RLock()
			activeCount := len(m.activeSessions)
			for id, session := range m.activeSessions {
				progress := session.GetProgress()
				if err := m.db.UpdateProgress(progress); err != nil {
					logs.Warn("failed to update progress", "download_id", id, "error", err)
				}
			}
			m.mu.RUnlock()
			logs.Debug("sync complete", "active_sessions", activeCount)
		}
	}
}

func (m *Manager) startDownload(item model.Content) error {
	log := slog.With("platform", item.Platform, "video_id", item.VideoId, "title", item.Title)
	p, ok := m.providers[item.Platform]
	if !ok {
		return fmt.Errorf("provider not found: %s", item.Platform)
	}
	log.Info("starting download session")
	session, err := p.Download(item)
	if err != nil {
		return err
	}

	downloadID := item.DownloadID()

	m.mu.Lock()
	m.activeSessions[downloadID] = session
	m.mu.Unlock()
	log.Debug("download session registered", "download_id", downloadID)

	go func() {
		err := session.Wait()
		m.mu.Lock()
		delete(m.activeSessions, downloadID)
		m.mu.Unlock()

		if err != nil {
			log.Error("download session failed", "download_id", downloadID, "error", err)
			_ = m.db.SetStatus(item, "failed", err.Error())
		} else {
			log.Info("download session complete", "download_id", downloadID)
			_ = m.db.SetStatus(item, "complete", "")
		}
	}()

	return nil
}

func (m *Manager) stopActiveSessions() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	slog.Info("stopping active sessions", "count", len(m.activeSessions))
	for _, session := range m.activeSessions {
		session.Stop()
	}
}

