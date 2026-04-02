package manager

import (
	"context"
	"sync"
	"testing"

	"deps.me/dl-daemon/internal/model"
	"deps.me/dl-daemon/internal/platform"
)

type fakeSession struct {
	mu      sync.Mutex
	stopped bool
}

func (s *fakeSession) GetProgress() model.DownloadProgress {
	return model.DownloadProgress{}
}

func (s *fakeSession) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
}

func (s *fakeSession) Wait() error {
	<-make(chan struct{})
	return nil
}

func (s *fakeSession) Stopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

func TestRunStopsActiveSessionsOnContextCancel(t *testing.T) {
	session := &fakeSession{}
	mgr := &Manager{
		providers:      map[string]platform.Provider{},
		activeSessions: map[string]platform.DownloadSession{"vid-1": session},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := mgr.Run(ctx); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !session.Stopped() {
		t.Fatal("expected active session to be stopped on shutdown")
	}
}

func TestStartDownloadFailsWhenProviderMissing(t *testing.T) {
	mgr := &Manager{providers: map[string]platform.Provider{}, activeSessions: map[string]platform.DownloadSession{}}

	err := mgr.startDownload(model.Content{VideoId: "vid-1", Platform: "missing", Title: "title"})
	if err == nil {
		t.Fatal("expected missing provider error")
	}
}
