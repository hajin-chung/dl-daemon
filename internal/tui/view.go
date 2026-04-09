package tui

import (
	"fmt"
	"sort"
	"strings"

	"deps.me/dl-daemon/internal/db"
	"deps.me/dl-daemon/internal/model"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#5f5faf")).
			Padding(0, 1)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#87afaf"))

	statusComplete = lipgloss.NewStyle().Foreground(lipgloss.Color("#87af5f"))
	statusFailed   = lipgloss.NewStyle().Foreground(lipgloss.Color("#d75f5f"))
	statusActive   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffaf"))
	statusPending  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a8a8a"))

	barDone = lipgloss.NewStyle().Foreground(lipgloss.Color("#5f87af"))
	barFill = lipgloss.NewStyle().Foreground(lipgloss.Color("#4e4e4e"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c6c6c"))
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render(" dld "))
	b.WriteString("\n\n")

	b.WriteString(renderTargets(m.targets))
	b.WriteString("\n")

	overhead := 3 + len(m.targets) + 2 + downloadHeaderLines() + 2
	if len(m.targets) == 0 {
		overhead += 1
	}
	if len(m.downloads) == 0 {
		overhead += 1
	}
	availHeight := m.height - overhead
	if availHeight < 1 {
		availHeight = 1
	}

	b.WriteString(renderDownloads(m.downloads, m.scrollOffset, m.width, availHeight))
	b.WriteString("\n")

	b.WriteString(renderFooter(m.downloads, m.err))

	return b.String()
}

func renderTargets(targets []model.Target) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render(fmt.Sprintf("TARGETS (%d)", len(targets))))
	b.WriteByte('\n')

	if len(targets) == 0 {
		b.WriteString("  No targets configured.\n")
		return b.String()
	}

	for _, t := range targets {
		label := t.Label
		if label == "" {
			label = t.Id
		}
		if t.OutputDir != "" {
			b.WriteString(fmt.Sprintf("  %-12s %-20s %s\n", t.Platform, label, t.OutputDir))
		} else {
			b.WriteString(fmt.Sprintf("  %-12s %s\n", t.Platform, label))
		}
	}

	return b.String()
}

func sortDownloads(downloads []db.DownloadRow) []db.DownloadRow {
	statusRank := map[string]int{
		"downloading": 0,
		"starting":    1,
		"pending":     2,
		"failed":      3,
		"complete":    4,
	}
	sort.SliceStable(downloads, func(i, j int) bool {
		ri, ok := statusRank[downloads[i].Status]
		if !ok {
			ri = 5
		}
		rj, ok := statusRank[downloads[j].Status]
		if !ok {
			rj = 5
		}
		return ri < rj
	})
	return downloads
}

func downloadHeaderLines() int {
	return 4
}

func renderDownloads(downloads []db.DownloadRow, scrollOffset int, width int, availHeight int) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render(fmt.Sprintf("DOWNLOADS (%d)", len(downloads))))
	b.WriteByte('\n')

	if len(downloads) == 0 {
		b.WriteString("  No downloads.\n")
		return b.String()
	}

	active, complete, failed := countStatuses(downloads)
	b.WriteString(fmt.Sprintf("  %s %d active · %s %d complete · %s %d failed\n",
		statusActive.Render("●"), active,
		statusComplete.Render("●"), complete,
		statusFailed.Render("●"), failed,
	))

	visibleRows := availHeight - downloadHeaderLines()
	if visibleRows < 1 {
		visibleRows = 1
	}
	maxOffset := max(0, len(downloads)-visibleRows)
	if scrollOffset > maxOffset {
		scrollOffset = maxOffset
	}

	end := scrollOffset + visibleRows
	if end > len(downloads) {
		end = len(downloads)
	}

	b.WriteString(fmt.Sprintf("  %d-%d of %d\n", scrollOffset+1, end, len(downloads)))
	b.WriteByte('\n')

	barWidth := 20
	if width > 0 && width < 80 {
		barWidth = 10
	}

	for _, dl := range downloads[scrollOffset:end] {
		status := statusIcon(dl.Status)
		pct := progressPct(dl.BytesWritten, dl.TotalBytes)
		bar := progressBar(pct, barWidth)
		size := formatSize(dl.BytesWritten, dl.TotalBytes)

		title := dl.Title
		if title == "" {
			title = dl.VideoID
		}
		if len(title) > 40 {
			title = title[:37] + "..."
		}

		b.WriteString(fmt.Sprintf("  %s %-12s %s %s %s\n",
			status, dl.Platform, bar, size, title))
	}

	return b.String()
}

func renderFooter(downloads []db.DownloadRow, errMsg string) string {
	if errMsg != "" {
		return statusFailed.Render("ERR "+errMsg) + "\n" + footerStyle.Render("q: quit · r: refresh · j/k: scroll")
	}
	return footerStyle.Render("q: quit · r: refresh · j/k: scroll")
}

func statusIcon(status string) string {
	switch status {
	case "complete":
		return statusComplete.Render("✓")
	case "failed":
		return statusFailed.Render("✗")
	case "downloading", "starting":
		return statusActive.Render("▸")
	default:
		return statusPending.Render("○")
	}
}

func countStatuses(downloads []db.DownloadRow) (active, complete, failed int) {
	for _, dl := range downloads {
		switch dl.Status {
		case "complete":
			complete++
		case "failed":
			failed++
		case "pending", "starting", "downloading":
			active++
		}
	}
	return
}

func progressPct(written, total int64) float64 {
	if total <= 0 {
		return 0
	}
	pct := float64(written) / float64(total)
	if pct > 1 {
		pct = 1
	}
	return pct
}

func progressBar(pct float64, width int) string {
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)
	return barDone.Render(bar[:filled]) + barFill.Render(bar[filled:])
}

func formatSize(written, total int64) string {
	if total > 0 {
		return fmt.Sprintf("%s/%s", humanBytes(written), humanBytes(total))
	}
	if written > 0 {
		return humanBytes(written)
	}
	return ""
}

func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	value := float64(n)
	for _, unit := range units {
		value /= 1024
		if value < 1024 {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%.1f PiB", value/1024)
}
