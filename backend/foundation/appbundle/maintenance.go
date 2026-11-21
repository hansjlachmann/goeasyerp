package appbundle

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
)

// Maintenance: while the apps of a new server version are being built, the server answers every
// API request with 503 and a message (the login page shows it), /health with 200 — the
// container is up, probes do not restart it —, and starts the bundle as soon as it is built.

// MaintenanceStatus: what the maintenance message tells
type MaintenanceStatus struct {
	mu      sync.Mutex
	failed   string // the error of the last build of the version, when it failed
	version  string
	starting bool // a switch to a newer build: the program is starting (ServeStarting)
}

func (s *MaintenanceStatus) set(failed string) {
	s.mu.Lock()
	s.failed = failed
	s.mu.Unlock()
}

// maintenanceAnswer: the API's answer during maintenance. The page cannot load its texts from
// the server then, so the answer brings the notice's texts in the browser's language: its
// heading, the message and the Try again button's caption.
type maintenanceAnswer struct {
	Success     bool   `json:"success"`
	Error       string `json:"error"`       // the message (also what clients show as the error)
	Maintenance bool   `json:"maintenance"` // the marker: maintenance, not an error of the request
	Failed      bool   `json:"failed"`      // the build of the apps failed: an administrator acts
	Starting    bool   `json:"starting"`    // the server is starting a newer build: back in moments
	Title       string `json:"title"`
	Retry       string `json:"retry"`
}

func (s *MaintenanceStatus) answer(lang string) maintenanceAnswer {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := i18n.GetInstance()
	a := maintenanceAnswer{Maintenance: true, Retry: ts.Message("APP_MAINTENANCE_RETRY", lang)}
	if s.starting {
		a.Starting = true
		a.Title = ts.Message("APP_MAINTENANCE_TITLE", lang)
		a.Error = ts.Message("APP_MAINTENANCE_STARTING", lang)
		return a
	}
	if s.failed != "" {
		a.Failed = true
		a.Title = ts.Message("APP_MAINTENANCE_TITLE_FAILED", lang)
		a.Error = ts.MessageWithParams("APP_MAINTENANCE_BUILD_FAILED", lang, s.version, s.failed)
		return a
	}
	a.Title = ts.Message("APP_MAINTENANCE_TITLE", lang)
	a.Error = ts.MessageWithParams("APP_MAINTENANCE_BUILDING", lang, s.version)
	return a
}

// maintenanceHandler answers the requests during maintenance
func maintenanceHandler(status *MaintenanceStatus) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "30")
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "maintenance"})
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(status.answer(language(r.Header.Get("Accept-Language"))))
	})
}

// language: the first supported language the browser accepts (en-US otherwise)
func language(header string) string {
	for _, part := range strings.Split(header, ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag = strings.ToLower(tag)
		switch {
		case strings.HasPrefix(tag, "nb"), strings.HasPrefix(tag, "no"), strings.HasPrefix(tag, "nn"):
			return "nb-NO"
		case strings.HasPrefix(tag, "da"):
			return "da-DK"
		case strings.HasPrefix(tag, "en"):
			return "en-US"
		}
	}
	return "en-US"
}

// lastFailure: the error of the newest build of the version when it failed (none when a build
// is requested or running after it)
func lastFailure(db *sql.DB, version string) string {
	var status int
	var message sql.NullString
	err := db.QueryRow(`SELECT status, error_message FROM "App_Build" WHERE base_version = $1 ORDER BY entry_no DESC LIMIT 1`, version).Scan(&status, &message)
	if err != nil || status != statusFailed {
		return ""
	}
	if message.String == "" {
		return "-"
	}
	return message.String
}

// statusFailed: App Build Status Failed
const statusFailed = 3

// ServeMaintenance answers requests on ln in maintenance and checks every interval for the
// bundle; when it is there it hands the socket over (HandOver) and returns the bundle (the
// caller starts it). PostgreSQL only.
func ServeMaintenance(ctx context.Context, db *sql.DB, ln net.Listener, buildsDir, version, baseAppID string, interval time.Duration) (Decision, error) {
	status := &MaintenanceStatus{version: version}
	srv := &http.Server{Handler: maintenanceHandler(status), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		err := srv.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			log.Printf("maintenance server: %v", err)
		}
	}()
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	for {
		d, err := Decide(db, database.DBTypePostgres, buildsDir, version, baseAppID)
		if err != nil {
			log.Printf("maintenance: %v", err)
		} else if !d.Maintenance {
			// before the deferred shutdown closes the listener
			return d, HandOver(ln)
		}
		status.set(lastFailure(db, version))
		select {
		case <-ctx.Done():
			return Decision{}, ctx.Err()
		case <-time.After(interval):
		}
	}
}
