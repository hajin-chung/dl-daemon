package chzzk_live

import (
	"context"
	"sync"

	"deps.me/dl-daemon/internal/model"
)

type Session struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.RWMutex
	progress model.DownloadProgress
	err      error
	done     chan struct{}
}

func NewSession(videoID string, platform string) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		ctx:    ctx,
		cancel: cancel,
		progress: model.DownloadProgress{
			VideoId:  videoID,
			Platform: platform,
			Status:   "pending",
		},
		done: make(chan struct{}),
	}
}

func (s *Session) Context() context.Context { return s.ctx }
func (s *Session) UpdateStatus(status string) {
	s.mu.Lock(); defer s.mu.Unlock(); s.progress.Status = status
}
func (s *Session) SetBytes(bytesWritten int64, totalBytes int64) {
	s.mu.Lock(); defer s.mu.Unlock(); s.progress.BytesWritten = bytesWritten; s.progress.TotalBytes = totalBytes
}
func (s *Session) GetProgress() model.DownloadProgress {
	s.mu.RLock(); defer s.mu.RUnlock(); return s.progress
}
func (s *Session) Stop() { s.cancel() }
func (s *Session) Finish(err error) { s.mu.Lock(); s.err = err; s.mu.Unlock(); close(s.done) }
func (s *Session) Wait() error { <-s.done; s.mu.RLock(); defer s.mu.RUnlock(); return s.err }
