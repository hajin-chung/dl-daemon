package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tick()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			return m.refresh()
		case "j", "down":
			if m.scrollOffset < max(0, len(m.downloads)-1) {
				m.scrollOffset++
			}
			return m, nil
		case "k", "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		}

	case tickMsg:
		return m.refresh()
	}

	return m, nil
}

func (m Model) refresh() (tea.Model, tea.Cmd) {
	targets, err := m.db.GetTargets()
	if err != nil {
		m.err = err.Error()
		return m, tick()
	}
	m.targets = targets

	downloads, err := m.db.ListDownloads()
	if err != nil {
		m.err = err.Error()
		return m, tick()
	}
	m.downloads = sortDownloads(downloads)
	m.err = ""

	maxOffset := max(0, len(m.downloads)-1)
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}

	return m, tick()
}
