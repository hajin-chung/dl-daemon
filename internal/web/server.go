package web

import (
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"deps.me/dl-daemon/internal/db"
	"deps.me/dl-daemon/internal/model"
)

//go:embed templates/*.html
var templateFS embed.FS

type Server struct {
	db        *db.DB
	templates *template.Template
}

type ViewData struct {
	Title            string
	CurrentPath      string
	Flash            string
	Error            string
	Now              time.Time
	Dashboard        DashboardData
	Targets          []dbTargetView
	Downloads        []dbDownloadView
	Config           []configView
	PlatformOptions  []string
	MaskedConfigKeys map[string]bool
}

type DashboardData struct {
	TargetCount          int
	DownloadCount        int
	ActiveDownloadCount  int
	CompletedCount       int
	FailedCount          int
	ConfigCount          int
	TargetsByPlatform    []platformCount
	RecentDownloads      []dbDownloadView
	HasConfiguredTargets bool
}

type platformCount struct {
	Name  string
	Count int
}

type dbTargetView struct {
	Platform  string
	ID        string
	Label     string
	OutputDir string
}

type dbDownloadView struct {
	VideoID      string
	Title        string
	Platform     string
	Status       string
	BytesWritten int64
	TotalBytes   int64
	ErrorMsg     string
	ProgressText string
}

type configView struct {
	Key         string
	Value       string
	MaskedValue string
	Sensitive   bool
}

func New(database *db.DB) (*Server, error) {
	funcs := template.FuncMap{
		"navActive": func(current, want string) bool {
			return current == want
		},
	}

	tmpl, err := template.New("base").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Server{db: database, templates: tmpl}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.dashboard)
	mux.HandleFunc("GET /targets", s.targetsPage)
	mux.HandleFunc("POST /targets", s.addTarget)
	mux.HandleFunc("POST /targets/delete", s.deleteTarget)
	mux.HandleFunc("POST /targets/output", s.updateTargetOutput)
	mux.HandleFunc("GET /downloads", s.downloadsPage)
	mux.HandleFunc("GET /config", s.configPage)
	mux.HandleFunc("POST /config", s.setConfig)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return s.logging(mux)
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Debug("web request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	view, err := s.loadViewData(r)
	if err != nil {
		s.renderError(w, err)
		return
	}
	view.Title = "Dashboard"
	view.Dashboard = buildDashboard(view)
	if err := s.render(w, "dashboard", view); err != nil {
		s.renderError(w, err)
	}
}

func (s *Server) targetsPage(w http.ResponseWriter, r *http.Request) {
	view, err := s.loadViewData(r)
	if err != nil {
		s.renderError(w, err)
		return
	}
	view.Title = "Targets"
	if err := s.render(w, "targets", view); err != nil {
		s.renderError(w, err)
	}
}

func (s *Server) downloadsPage(w http.ResponseWriter, r *http.Request) {
	view, err := s.loadViewData(r)
	if err != nil {
		s.renderError(w, err)
		return
	}
	view.Title = "Downloads"
	if err := s.render(w, "downloads", view); err != nil {
		s.renderError(w, err)
	}
}

func (s *Server) configPage(w http.ResponseWriter, r *http.Request) {
	view, err := s.loadViewData(r)
	if err != nil {
		s.renderError(w, err)
		return
	}
	view.Title = "Config"
	if err := s.render(w, "config", view); err != nil {
		s.renderError(w, err)
	}
}

func (s *Server) addTarget(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/targets", err)
		return
	}
	platform := strings.TrimSpace(r.FormValue("platform"))
	id := strings.TrimSpace(r.FormValue("id"))
	label := strings.TrimSpace(r.FormValue("label"))
	outputDir := strings.TrimSpace(r.FormValue("output_dir"))
	if platform == "" || id == "" {
		s.redirectWithError(w, r, "/targets", "platform and id are required")
		return
	}
	if err := s.db.AddTarget(model.Target{Platform: platform, Id: id, Label: label, OutputDir: outputDir}); err != nil {
		s.redirectError(w, r, "/targets", err)
		return
	}
	redirectWithMessage(w, r, "/targets", "Target added.")
}

func (s *Server) deleteTarget(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/targets", err)
		return
	}
	platform := strings.TrimSpace(r.FormValue("platform"))
	id := strings.TrimSpace(r.FormValue("id"))
	if platform == "" || id == "" {
		s.redirectWithError(w, r, "/targets", "platform and id are required")
		return
	}
	if err := s.db.RemoveTarget(platform, id); err != nil {
		s.redirectError(w, r, "/targets", err)
		return
	}
	redirectWithMessage(w, r, "/targets", "Target removed.")
}

func (s *Server) updateTargetOutput(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/targets", err)
		return
	}
	platform := strings.TrimSpace(r.FormValue("platform"))
	id := strings.TrimSpace(r.FormValue("id"))
	outputDir := strings.TrimSpace(r.FormValue("output_dir"))
	if platform == "" || id == "" {
		s.redirectWithError(w, r, "/targets", "platform and id are required")
		return
	}
	if err := s.db.SetTargetOutputDir(platform, id, outputDir); err != nil {
		s.redirectError(w, r, "/targets", err)
		return
	}
	redirectWithMessage(w, r, "/targets", "Target output directory updated.")
}

func (s *Server) setConfig(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/config", err)
		return
	}
	key := strings.TrimSpace(r.FormValue("key"))
	value := strings.TrimSpace(r.FormValue("value"))
	if key == "" {
		s.redirectWithError(w, r, "/config", "config key is required")
		return
	}
	if err := s.db.SetMetadata(key, value); err != nil {
		s.redirectError(w, r, "/config", err)
		return
	}
	redirectWithMessage(w, r, "/config", fmt.Sprintf("Saved %s.", key))
}

func (s *Server) loadViewData(r *http.Request) (ViewData, error) {
	targets, err := s.db.GetTargets()
	if err != nil {
		return ViewData{}, err
	}
	downloads, err := s.db.ListDownloads()
	if err != nil {
		return ViewData{}, err
	}
	metadata, err := s.db.ListMetadata()
	if err != nil {
		return ViewData{}, err
	}

	view := ViewData{
		CurrentPath:      r.URL.Path,
		Now:              time.Now().UTC(),
		Targets:          make([]dbTargetView, 0, len(targets)),
		Downloads:        make([]dbDownloadView, 0, len(downloads)),
		Config:           make([]configView, 0, len(metadata)),
		PlatformOptions:  []string{"chzzk", "chzzk_live", "anilife"},
		MaskedConfigKeys: map[string]bool{},
	}

	for _, t := range targets {
		view.Targets = append(view.Targets, dbTargetView{Platform: t.Platform, ID: t.Id, Label: t.Label, OutputDir: t.OutputDir})
	}
	for _, row := range downloads {
		errMsg := ""
		if row.ErrorMsg != nil {
			errMsg = *row.ErrorMsg
		}
		view.Downloads = append(view.Downloads, dbDownloadView{
			VideoID:      row.VideoID,
			Title:        row.Title,
			Platform:     row.Platform,
			Status:       row.Status,
			BytesWritten: row.BytesWritten,
			TotalBytes:   row.TotalBytes,
			ErrorMsg:     errMsg,
			ProgressText: formatProgress(row.BytesWritten, row.TotalBytes),
		})
	}
	for _, row := range metadata {
		sensitive := isSensitiveKey(row.Key)
		view.MaskedConfigKeys[row.Key] = sensitive
		view.Config = append(view.Config, configView{
			Key:         row.Key,
			Value:       row.Value,
			MaskedValue: maskConfigValue(row.Key, row.Value),
			Sensitive:   sensitive,
		})
	}
	ApplyFlash(r, &view)
	return view, nil
}

func buildDashboard(view ViewData) DashboardData {
	counts := map[string]int{}
	d := DashboardData{
		TargetCount:          len(view.Targets),
		DownloadCount:        len(view.Downloads),
		ConfigCount:          len(view.Config),
		HasConfiguredTargets: len(view.Targets) > 0,
	}
	for _, t := range view.Targets {
		counts[t.Platform]++
	}
	for _, dl := range view.Downloads {
		switch dl.Status {
		case "complete":
			d.CompletedCount++
		case "failed":
			d.FailedCount++
		case "pending", "starting", "downloading":
			d.ActiveDownloadCount++
		}
	}
	for name, count := range counts {
		d.TargetsByPlatform = append(d.TargetsByPlatform, platformCount{Name: name, Count: count})
	}
	sort.Slice(d.TargetsByPlatform, func(i, j int) bool {
		return d.TargetsByPlatform[i].Name < d.TargetsByPlatform[j].Name
	})
	if len(view.Downloads) > 8 {
		d.RecentDownloads = append(d.RecentDownloads, view.Downloads[:8]...)
	} else {
		d.RecentDownloads = append(d.RecentDownloads, view.Downloads...)
	}
	return d
}

func (s *Server) render(w http.ResponseWriter, name string, view ViewData) error {
	view.Flash = strings.TrimSpace(view.Flash)
	view.Error = strings.TrimSpace(view.Error)
	return s.templates.ExecuteTemplate(w, name, view)
}

func (s *Server) renderError(w http.ResponseWriter, err error) {
	slog.Error("web request failed", "error", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (s *Server) redirectError(w http.ResponseWriter, r *http.Request, path string, err error) {
	s.redirectWithError(w, r, path, err.Error())
}

func (s *Server) redirectWithError(w http.ResponseWriter, r *http.Request, path string, message string) {
	http.Redirect(w, r, path+"?error="+url.QueryEscape(message), http.StatusSeeOther)
}

func redirectWithMessage(w http.ResponseWriter, r *http.Request, path string, message string) {
	http.Redirect(w, r, path+"?msg="+url.QueryEscape(message), http.StatusSeeOther)
}

func formatProgress(written, total int64) string {
	if total > 0 {
		return fmt.Sprintf("%s / %s", humanBytes(written), humanBytes(total))
	}
	if written > 0 {
		return humanBytes(written)
	}
	return "—"
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

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "token") || strings.Contains(lower, "aut") || strings.Contains(lower, "ses") || strings.Contains(lower, "secret") || strings.Contains(lower, "password")
}

func maskConfigValue(key string, value string) string {
	if !isSensitiveKey(key) {
		return value
	}
	if len(value) <= 8 {
		return "********"
	}
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}

func ApplyFlash(r *http.Request, view *ViewData) {
	view.Flash = strings.TrimSpace(r.URL.Query().Get("msg"))
	view.Error = strings.TrimSpace(r.URL.Query().Get("error"))
}

