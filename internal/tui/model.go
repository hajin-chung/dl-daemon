package tui

import (
	"deps.me/dl-daemon/internal/db"
	"deps.me/dl-daemon/internal/model"
)

type Model struct {
	db           *db.DB
	width        int
	height       int
	targets      []model.Target
	downloads    []db.DownloadRow
	scrollOffset int
	err          string
	quitting     bool
}

func New(database *db.DB) Model {
	return Model{
		db: database,
	}
}
