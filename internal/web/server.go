package web

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/app"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
	"github.com/funnelscript/rem-from-funnelscript/internal/report"
)

//go:embed index.html
var page string

//go:embed app.css
var stylesheet string

//go:embed app.js
var script string

const (
	maxRequestBytes = 64 << 10
	maxActiveJobs   = 2
	maxRemembered   = 64
	jobTimeout      = 4 * time.Minute
	jobRetention    = 30 * time.Minute
)

type backend struct {
	doctor      func() map[string]any
	investigate func(context.Context, string, app.Options) (model.Bundle, error)
	list        func() ([]model.Summary, error)
	load        func(string) (model.Bundle, error)
	report      func(string) (string, error)
	latestPair  func(string) (model.Bundle, model.Bundle, error)
}

type jobStatus struct {
	State  string            `json:"state"`
	RunID  string            `json:"runId,omitempty"`
	Stages map[string]string `json:"stages"`
	Error  string            `json:"error,omitempty"`
}

type job struct {
	jobStatus
	cancel   context.CancelFunc
	finished time.Time
}

type server struct {
	backend
	mu   sync.Mutex
	jobs map[string]*job
}

// New exposes the local UI and run API. Serve the handler on IPv4 loopback.
func New(service *app.Service) http.Handler {
	return newHandler(backend{
		doctor: service.Doctor, investigate: service.Investigate,
		list: service.Store.List, load: service.Store.Load, report: service.Store.Report,
		latestPair: service.Store.LatestPair,
	})
}

func newHandler(b backend) http.Handler {
	s := &server{backend: b, jobs: make(map[string]*job)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", asset("text/html; charset=utf-8", page))
	mux.HandleFunc("GET /app.css", asset("text/css; charset=utf-8", stylesheet))
	mux.HandleFunc("GET /app.js", asset("text/javascript; charset=utf-8", script))
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.doctor())
	})
	mux.HandleFunc("GET /api/runs", s.listRuns)
	mux.HandleFunc("POST /api/runs", s.startRun)
	mux.HandleFunc("GET /api/runs/{id}", s.getRun)
	mux.HandleFunc("GET /api/runs/{id}/report.md", s.downloadReport)
	mux.HandleFunc("GET /api/runs/{id}/evidence.json", s.downloadEvidence)
	mux.HandleFunc("GET /api/compare", s.compareRuns)
	mux.HandleFunc("GET /api/jobs/{id}", s.getJob)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", s.cancelJob)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https: http:; media-src https: http:; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if !localRequest(r) {
			problem(w, http.StatusForbidden, "use the app's local address")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func localRequest(r *http.Request) bool {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host, port = r.Host, "80"
	}
	if !strings.EqualFold(host, "localhost") && host != "127.0.0.1" {
		return false
	}
	if n, err := strconv.ParseUint(port, 10, 16); err != nil || n == 0 {
		return false
	}
	if addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		_, listenPort, err := net.SplitHostPort(addr.String())
		if err != nil || port != listenPort {
			return false
		}
	}
	origin := r.Header.Get("Origin")
	return origin == "" || strings.EqualFold(origin, "http://"+r.Host)
}

func asset(contentType, data string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = io.WriteString(w, data)
	}
}

func problem(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *server) listRuns(w http.ResponseWriter, _ *http.Request) {
	runs, err := s.list()
	if err != nil {
		problem(w, http.StatusInternalServerError, "could not list saved runs")
		return
	}
	if runs == nil {
		runs = []model.Summary{}
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *server) getRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bundle, err := s.load(id)
	if err != nil {
		storageError(w, err)
		return
	}
	markdown, err := s.report(id)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		problem(w, http.StatusInternalServerError, "could not open the report")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Bundle model.Bundle `json:"bundle"`
		Report string       `json:"report"`
	}{Bundle: bundle, Report: markdown})
}

func (s *server) downloadReport(w http.ResponseWriter, r *http.Request) {
	markdown, err := s.report(r.PathValue("id"))
	if err != nil {
		storageError(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="report.md"`)
	asset("text/markdown; charset=utf-8", markdown)(w, r)
}

func (s *server) downloadEvidence(w http.ResponseWriter, r *http.Request) {
	bundle, err := s.load(r.PathValue("id"))
	if err != nil {
		storageError(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="evidence.json"`)
	writeJSON(w, http.StatusOK, bundle)
}

func (s *server) compareRuns(w http.ResponseWriter, r *http.Request) {
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	if domain == "" || len(domain) > 253 {
		problem(w, http.StatusBadRequest, "enter a domain to compare")
		return
	}
	a, b, err := s.latestPair(domain)
	if err != nil {
		if strings.HasPrefix(err.Error(), "need two saved runs") {
			problem(w, http.StatusNotFound, "save two investigations of this domain to compare changes")
			return
		}
		problem(w, http.StatusInternalServerError, "could not compare saved runs")
		return
	}
	writeJSON(w, http.StatusOK, report.Compare(a, b))
}

func storageError(w http.ResponseWriter, err error) {
	if errors.Is(err, fs.ErrNotExist) {
		problem(w, http.StatusNotFound, "saved run not found")
		return
	}
	problem(w, http.StatusInternalServerError, "could not read the saved run")
}

type runInput struct {
	URL        string            `json:"url"`
	Only       []string          `json:"only,omitempty"`
	Handles    map[string]string `json:"handles,omitempty"`
	MaxCredits *int              `json:"maxCredits,omitempty"`
	Engine     string            `json:"engine,omitempty"`
	Model      string            `json:"model,omitempty"`
}

func (s *server) startRun(w http.ResponseWriter, r *http.Request) {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		problem(w, http.StatusUnsupportedMediaType, "use application/json")
		return
	}
	body := http.MaxBytesReader(w, r.Body, maxRequestBytes)
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var input runInput
	err = decoder.Decode(&input)
	if err == nil {
		var extra any
		err = decoder.Decode(&extra)
		if errors.Is(err, io.EOF) {
			err = nil
		} else if err == nil {
			err = errors.New("extra JSON value")
		}
	}
	var oversized *http.MaxBytesError
	switch {
	case errors.As(err, &oversized):
		problem(w, http.StatusRequestEntityTooLarge, "request is too large")
		return
	case err != nil:
		problem(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	input.URL = strings.TrimSpace(input.URL)
	if input.URL == "" || len(input.URL) > 2048 {
		problem(w, http.StatusBadRequest, "enter a website or app store link")
		return
	}
	if input.Engine == "" {
		input.Engine = "evidence"
	}
	switch input.Engine {
	case "evidence", "anthropic", "claude-code", "codex":
	default:
		problem(w, http.StatusBadRequest, "unknown report writer")
		return
	}
	if input.Engine != "evidence" {
		engines, _ := s.doctor()["engines"].(map[string]bool)
		if !engines[input.Engine] {
			problem(w, http.StatusBadRequest, "the selected report writer is unavailable")
			return
		}
	}
	opts := app.Options{Only: input.Only, Handles: input.Handles, Engine: input.Engine, Model: input.Model}
	if input.MaxCredits != nil {
		opts.BudgetSet = true
		opts.MaxCredits = *input.MaxCredits
	}
	if _, err := app.ParseTarget(input.URL); err != nil {
		problem(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := app.CheckOptions(opts); err != nil {
		problem(w, http.StatusBadRequest, err.Error())
		return
	}
	id := s.startJob(input.URL, opts)
	if id == "" {
		problem(w, http.StatusTooManyRequests, "two investigations are already running")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": id})
}

func (s *server) startJob(target string, opts app.Options) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	var oldestID string
	var oldest time.Time
	active := 0
	for id, j := range s.jobs {
		switch {
		case j.finished.IsZero():
			active++
		case now.Sub(j.finished) > jobRetention:
			delete(s.jobs, id)
		case oldestID == "" || j.finished.Before(oldest):
			oldestID, oldest = id, j.finished
		}
	}
	if active >= maxActiveJobs {
		return ""
	}
	if len(s.jobs) >= maxRemembered {
		delete(s.jobs, oldestID)
	}
	id := hex.EncodeToString(randBytes())
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	j := &job{jobStatus: jobStatus{State: "running", Stages: map[string]string{}}, cancel: cancel}
	s.jobs[id] = j
	opts.OnStage = func(stage, state string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		j.Stages[stage] = state
	}
	go func() {
		defer cancel()
		bundle, err := s.investigate(ctx, target, opts)
		s.mu.Lock()
		defer s.mu.Unlock()
		j.finished = time.Now()
		switch {
		case err == nil:
			j.State, j.RunID = "done", bundle.ID
		case errors.Is(ctx.Err(), context.Canceled):
			j.State = "cancelled"
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			j.State, j.Error = "failed", "the investigation reached its time limit"
		default:
			j.State, j.Error = "failed", err.Error()
		}
	}()
	return id
}

func randBytes() []byte {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return b[:]
}

func (s *server) getJob(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	j := s.jobs[r.PathValue("id")]
	if j == nil || (!j.finished.IsZero() && time.Since(j.finished) > jobRetention) {
		s.mu.Unlock()
		problem(w, http.StatusNotFound, "investigation job not found")
		return
	}
	status := j.jobStatus
	status.Stages = make(map[string]string, len(j.Stages))
	for k, v := range j.Stages {
		status.Stages[k] = v
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, status)
}

func (s *server) cancelJob(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	j := s.jobs[r.PathValue("id")]
	if j != nil && j.finished.IsZero() {
		j.cancel()
	}
	s.mu.Unlock()
	if j == nil {
		problem(w, http.StatusNotFound, "investigation job not found")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}
