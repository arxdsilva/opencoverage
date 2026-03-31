package main

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

//go:embed web/index.html web/assets/*
var embeddedFrontend embed.FS

type config struct {
	Addr            string
	APIBaseURL      string
	APIKeyHeader    string
	APIKeySecret    string
	AppVersion      string
	LatestVersion   string
	AppCommitSHA    string
	LatestCommitSHA string
	UpgradeURL      string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := loadConfig()

	frontendFS, err := fs.Sub(embeddedFrontend, "web")
	if err != nil {
		slog.Error("frontend_start_failed", "stage", "load_static_files", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/", http.FileServer(http.FS(frontendFS))))
	mux.HandleFunc("/api/app-meta", appMetaHandler(cfg))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		serveEmbeddedFile(w, http.FS(frontendFS), "index.html")
	})

	mux.HandleFunc("/api/projects", proxyHandler(cfg))
	mux.HandleFunc("/api/projects/", proxyHandler(cfg))

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      requestLogger(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("frontend_starting", "addr", cfg.Addr, "api_base_url", cfg.APIBaseURL)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("frontend_server_failed", "error", err)
		os.Exit(1)
	}
}

func loadConfig() config {
	cfg := config{
		Addr:            envOrDefault("FRONTEND_ADDR", ":8090"),
		APIBaseURL:      strings.TrimRight(envOrDefault("API_BASE_URL", "http://localhost:8080"), "/"),
		APIKeyHeader:    envOrDefault("API_KEY_HEADER", "X-API-Key"),
		APIKeySecret:    envOrDefault("API_KEY_SECRET", "dev-local-key"),
		AppVersion:      strings.TrimSpace(os.Getenv("APP_VERSION")),
		LatestVersion:   strings.TrimSpace(os.Getenv("APP_LATEST_VERSION")),
		AppCommitSHA:    strings.TrimSpace(os.Getenv("APP_COMMIT_SHA")),
		LatestCommitSHA: strings.TrimSpace(os.Getenv("APP_LATEST_COMMIT_SHA")),
		UpgradeURL:      envOrDefault("APP_UPGRADE_URL", "https://github.com/arxdsilva/opencoverage/releases"),
	}

	applyBuildVersionFallbacks(&cfg)
	return cfg
}

func applyBuildVersionFallbacks(cfg *config) {
	if cfg == nil {
		return
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
			if setting.Key == "vcs.revision" && cfg.AppCommitSHA == "" {
				cfg.AppCommitSHA = setting.Value
			}
			if setting.Key == "vcs.tag" && cfg.AppVersion == "" {
				cfg.AppVersion = setting.Value
			}
		}

		if cfg.AppVersion == "" && buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
			cfg.AppVersion = buildInfo.Main.Version
		}
	}

	if cfg.AppVersion == "" {
		if cfg.AppCommitSHA == "" {
			cfg.AppCommitSHA = resolveGitCommitSHA()
		}
		cfg.AppVersion = commitLabel(cfg.AppCommitSHA)
	}
}

func resolveGitCommitSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func proxyHandler(cfg config) http.HandlerFunc {
	client := &http.Client{Timeout: 20 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		target := cfg.APIBaseURL + "/v1" + strings.TrimPrefix(r.URL.Path, "/api")
		u, err := url.Parse(target)
		if err != nil {
			http.Error(w, "invalid target url", http.StatusInternalServerError)
			return
		}
		u.RawQuery = r.URL.RawQuery

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
		if err != nil {
			http.Error(w, "failed to build request", http.StatusInternalServerError)
			return
		}
		req.Header.Set(cfg.APIKeyHeader, cfg.APIKeySecret)

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "upstream request failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if ct := resp.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

type appMetaResponse struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	UpgradeURL     string `json:"upgradeUrl,omitempty"`
	HasUpgrade     bool   `json:"hasUpgrade"`
	Mode           string `json:"mode"`
}

func appMetaHandler(cfg config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		currentVersion, latestVersion, hasUpgrade, mode := resolveAppVersionInfo(cfg)

		writeJSON(w, http.StatusOK, appMetaResponse{
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			UpgradeURL:     cfg.UpgradeURL,
			HasUpgrade:     hasUpgrade,
			Mode:           mode,
		})
	}
}

func resolveAppVersionInfo(cfg config) (currentVersion string, latestVersion string, hasUpgrade bool, mode string) {
	if cfg.LatestVersion != "" {
		current := cfg.AppVersion
		if current == "" {
			current = "v-unknown"
		}
		return current, cfg.LatestVersion, isNewerVersion(cfg.LatestVersion, current), "release"
	}

	current := commitLabel(cfg.AppCommitSHA)
	if current == "v-unknown" && cfg.AppVersion != "" {
		current = cfg.AppVersion
	}
	latest := commitLabel(cfg.LatestCommitSHA)

	hasUpdateByCommit := cfg.AppCommitSHA != "" && cfg.LatestCommitSHA != "" && !strings.EqualFold(cfg.AppCommitSHA, cfg.LatestCommitSHA)
	if latest == "v-unknown" {
		latest = ""
	}

	return current, latest, hasUpdateByCommit, "commit"
}

func commitLabel(commit string) string {
	trimmed := strings.TrimSpace(commit)
	if trimmed == "" {
		return "v-unknown"
	}
	if len(trimmed) > 7 {
		trimmed = trimmed[:7]
	}
	return "v-" + trimmed
}

func isNewerVersion(latest, current string) bool {
	latest = strings.TrimSpace(strings.TrimPrefix(latest, "v"))
	current = strings.TrimSpace(strings.TrimPrefix(current, "v"))
	if latest == "" || current == "" || latest == current {
		return false
	}

	latestParts, latestOK := parseVersionParts(latest)
	currentParts, currentOK := parseVersionParts(current)
	if !latestOK || !currentOK {
		return latest != current
	}

	maxLen := len(latestParts)
	if len(currentParts) > maxLen {
		maxLen = len(currentParts)
	}

	for i := 0; i < maxLen; i++ {
		var latestPart, currentPart int
		if i < len(latestParts) {
			latestPart = latestParts[i]
		}
		if i < len(currentParts) {
			currentPart = currentParts[i]
		}
		if latestPart > currentPart {
			return true
		}
		if latestPart < currentPart {
			return false
		}
	}

	return false
}

func parseVersionParts(version string) ([]int, bool) {
	tokens := strings.FieldsFunc(version, func(r rune) bool {
		return r == '.' || r == '-' || r == '+' || r == '_'
	})
	if len(tokens) == 0 {
		return nil, false
	}

	parts := make([]int, 0, len(tokens))
	for _, token := range tokens {
		value, err := strconv.Atoi(token)
		if err != nil {
			return nil, false
		}
		parts = append(parts, value)
	}

	return parts, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("frontend_request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func serveEmbeddedFile(w http.ResponseWriter, fsys http.FileSystem, name string) {
	f, err := fsys.Open(name)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	_, _ = io.Copy(w, f)
}
