// TODO: add logging messages
// TODO: let time intervals be configurable

package manager

import (
	"context"
	"fmt"
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
		m.stopActiveSessions()
		return nil
	case err := <-watchErrCh:
		m.stopActiveSessions()
		if err == nil || err == context.Canceled {
			return nil
		}
		return fmt.Errorf("watch loop: %w", err)
	case err := <-syncErrCh:
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

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			targets, err := m.db.GetTargets()
			if err != nil {
				return err
			}
			for _, t := range targets {
				p, ok := m.providers[t.Platform]
				if !ok {
					continue
				}
				items, err := p.Watch(t.Id)
				if err != nil {
					continue
				}

				for _, item := range items {
					if !m.db.Exists(item) {
						if err := m.db.InsertDownload(item); err != nil {
							continue
						}
						if err := m.startDownload(item); err != nil {
							_ = m.db.SetStatus(item, "failed", err.Error())
						}
					}
				}
			}
		}
	}
}

func (m *Manager) syncLoop(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			m.mu.RLock()
			for _, session := range m.activeSessions {
				progress := session.GetProgress()
				_ = m.db.UpdateProgress(progress)
			}
			m.mu.RUnlock()
		}
	}
}

func (m *Manager) startDownload(item model.Content) error {
	p, ok := m.providers[item.Platform]
	if !ok {
		return fmt.Errorf("provider not found: %s", item.Platform)
	}
	session, err := p.Download(item)
	if err != nil {
		return err
	}

	downloadID := item.DownloadID()

	m.mu.Lock()
	m.activeSessions[downloadID] = session
	m.mu.Unlock()

	go func() {
		err := session.Wait()
		m.mu.Lock()
		delete(m.activeSessions, downloadID)
		m.mu.Unlock()

		if err != nil {
			_ = m.db.SetStatus(item, "failed", err.Error())
		} else {
			_ = m.db.SetStatus(item, "complete", "")
		}
	}()

	return nil
}

func (m *Manager) stopActiveSessions() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, session := range m.activeSessions {
		session.Stop()
	}
}

