package platform

import "deps.me/dl-daemon/internal/model"

type DownloadSession interface {
	GetProgress() model.DownloadProgress
	Stop()
	Wait() error // Blocks until download is finished
}

type Provider interface {
	Name() string
	// returns list of to be downloaded content
	Watch(id string) ([]model.Content, error)
	// start a downloading session
	Download(content model.Content) (DownloadSession, error)
}

