package cli

import (
	"context"
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kashifkhan0771/cuppa/internal/config"
	"github.com/kashifkhan0771/cuppa/internal/history"
	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// openAPI describes the HTTP API. A test checks it lists exactly the
// routes below, so the two can't drift apart.
//
//go:embed openapi.json
var openAPI []byte

const (
	apiVersion       = "v1"
	defaultListLimit = 100
	maxListLimit     = 1000
	postsPerMinute   = 10 // per client IP, for POST /v1/post
	shutdownWait     = 10 * time.Second
)

// runServe starts the HTTP API. It stops cleanly on Ctrl+C or SIGTERM,
// giving open requests a few seconds to finish.
func runServe(a *app, args []string) error {
	fs := a.newFlagSet("serve", "serve [--addr host:port] [--with-scheduler] [--insecure-allow-post]")
	addr := fs.String("addr", "", "address to listen on (default from config, else 127.0.0.1:8080)")
	withScheduler := fs.Bool("with-scheduler", false, "also post on schedule.cron, like the daemon")
	allowPost := fs.Bool("insecure-allow-post", false, "allow POST /v1/post when no server.token is set")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if err := a.loadConfig(); err != nil {
		return err
	}
	a.cfg.Server.Addr = listenAddr(*addr, a.cfg.Server.Addr)

	// build everything first, so a config mistake stops cuppa right away
	s, err := a.newServer(*allowPost)
	if err != nil {
		return err
	}
	var schedule func()
	if *withScheduler {
		plan, p, err := a.scheduledPoster()
		if err != nil {
			return err
		}
		schedule = func() { a.runSchedule(plan, p, false) }
	}

	listener, err := net.Listen("tcp", a.cfg.Server.Addr)
	if err != nil {
		return err
	}
	if !isLoopback(a.cfg.Server.Addr) && a.cfg.Server.Token == "" {
		a.log.Warn("listening beyond this computer with no server.token set; anyone who can reach it can use /v1/questions/next")
	}
	a.log.Info("listening", "addr", listener.Addr().String())

	srv := &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      2 * time.Minute, // a post retries slow chat services
		IdleTimeout:       time.Minute,
		ErrorLog:          slog.NewLogLogger(a.log.Handler(), slog.LevelWarn),
	}

	// the scheduler and the shutdown both end when cuppa is told to stop
	var background sync.WaitGroup
	if schedule != nil {
		background.Go(schedule)
	}
	background.Go(func() {
		<-a.ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), shutdownWait)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			a.log.Warn("some requests did not finish before shutdown", "error", err)
		}
	})

	err = srv.Serve(listener)
	background.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		a.log.Info("stopped")
		return nil
	}
	return err
}

// listenAddr is the --addr flag, else the config's addr. An empty
// addr (say, from an unset env var) would listen on every network,
// so it falls back to the safe default instead.
func listenAddr(flag, configured string) string {
	switch {
	case flag != "":
		return flag
	case configured != "":
		return configured
	default:
		return config.Defaults().Server.Addr
	}
}

// isLoopback reports whether an address only listens on this computer.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// server answers the HTTP API.
type server struct {
	a         *app
	all       *cuppa.Dataset // every question, private packs included
	allowPost bool
	limiter   *rateLimiter
}

func (a *app) newServer(allowPost bool) (*server, error) {
	all, err := a.allQuestions()
	if err != nil {
		return nil, err
	}
	return &server{
		a:         a,
		all:       all,
		allowPost: allowPost,
		limiter:   &rateLimiter{limit: postsPerMinute, now: a.now, windows: map[string]rateWindow{}},
	}, nil
}

// route is one API endpoint, in Go's "METHOD /path" pattern form.
type route struct {
	pattern string
	handle  http.HandlerFunc
}

func (s *server) routes() []route {
	return []route{
		{"GET /v1/questions/random", s.random},
		{"GET /v1/questions/today", s.today},
		{"GET /v1/questions/next", s.requireToken(s.next)},
		{"GET /v1/questions/{id}", s.question},
		{"GET /v1/questions", s.list},
		{"GET /v1/categories", s.categories},
		{"POST /v1/post", s.requireToken(s.post)},
		{"GET /healthz", s.health},
		{"GET /readyz", s.ready},
		{"GET /version", s.version},
		{"GET /openapi.json", s.openAPI},
	}
}

// handler is every route, wrapped with CORS, safety headers and a log
// line per request.
func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	for _, r := range s.routes() {
		mux.HandleFunc(r.pattern, r.handle)
	}
	return s.logRequests(s.cors(mux))
}

// replies

type envelope struct {
	Data any  `json:"data"`
	Meta meta `json:"meta"`
}

type meta struct {
	DatasetVersion string `json:"dataset_version"`
	APIVersion     string `json:"api_version"`
	// list only
	Total  int `json:"total,omitempty"`
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (s *server) meta() meta {
	return meta{DatasetVersion: s.all.Version(), APIVersion: apiVersion}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // the client went away; nothing to do
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var e apiError
	e.Error.Code, e.Error.Message = code, message
	writeJSON(w, status, e)
}

// writePick answers with a picked question, or why there was none.
func (s *server) writePick(w http.ResponseWriter, q cuppa.Question, err error) {
	switch {
	case errors.Is(err, cuppa.ErrNoMatch):
		writeError(w, http.StatusNotFound, "no_match", "no questions match the filter")
	case err != nil:
		s.a.log.Error("could not pick a question", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not pick a question")
	default:
		writeJSON(w, http.StatusOK, envelope{Data: q, Meta: s.meta()})
	}
}

// questions

// filtered is the config's selection narrowed by query parameters,
// the same way the command-line flags narrow it.
func (s *server) filtered(w http.ResponseWriter, r *http.Request) (*cuppa.Dataset, bool) {
	query := r.URL.Query()
	selection, err := s.a.selection(query.Get("category"), query.Get("tag"), "")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return nil, false
	}
	filter := selectionFilter(selection)
	if sfw := query.Get("sfw"); sfw != "" {
		if sfw != cuppa.SFWSafe && sfw != cuppa.SFWCasual {
			writeError(w, http.StatusBadRequest, "bad_request", "sfw must be safe or casual")
			return nil, false
		}
		filter.SFW = sfw
	}
	if locale := query.Get("locale"); locale != "" {
		filter.Locale = locale
	}
	filter.Types = splitList(query.Get("type"))
	return s.all.Filter(filter), true
}

func (s *server) random(w http.ResponseWriter, r *http.Request) {
	if d, ok := s.filtered(w, r); ok {
		q, err := cuppa.Random(d, nil)
		s.writePick(w, q, err)
	}
}

func (s *server) today(w http.ResponseWriter, r *http.Request) {
	d, ok := s.filtered(w, r)
	if !ok {
		return
	}
	seed := r.URL.Query().Get("seed")
	if seed == "" {
		seed = s.a.cfg.Selection.Seed
	}
	q, err := cuppa.Daily(d, seed, s.a.now().In(s.a.cfg.Location))
	s.writePick(w, q, err)
}

// next picks without repeats and saves it to the history, under the
// same lock as posting, so callers never get the same question.
func (s *server) next(w http.ResponseWriter, r *http.Request) {
	d, ok := s.filtered(w, r)
	if !ok {
		return
	}
	s.a.historyMu.Lock()
	q, err := cuppa.Next(d, history.NewFile(s.a.cfg.History.Path), nil)
	s.a.historyMu.Unlock()
	s.writePick(w, q, err)
}

func (s *server) question(w http.ResponseWriter, r *http.Request) {
	q, ok := s.all.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "no question with that id")
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: q, Meta: s.meta()})
}

func (s *server) list(w http.ResponseWriter, r *http.Request) {
	limit, err1 := queryInt(r, "limit", defaultListLimit)
	offset, err2 := queryInt(r, "offset", 0)
	if err1 != nil || err2 != nil || limit < 1 || limit > maxListLimit {
		writeError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("limit must be 1-%d and offset 0 or more", maxListLimit))
		return
	}
	d, ok := s.filtered(w, r)
	if !ok {
		return
	}

	// a page past the end is an empty list, not null
	all := d.All()
	page := append([]cuppa.Question{}, all[min(offset, len(all)):min(offset+limit, len(all))]...)
	m := s.meta()
	m.Total, m.Limit, m.Offset = len(all), limit, offset
	writeJSON(w, http.StatusOK, envelope{Data: page, Meta: m})
}

// queryInt reads a whole number 0 or more from the query, or fallback
// when it is missing.
func queryInt(r *http.Request, name string, fallback int) (int, error) {
	text := r.URL.Query().Get(name)
	if text == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a whole number", name)
	}
	return n, nil
}

func (s *server) categories(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, envelope{Data: s.all.Categories(), Meta: s.meta()})
}

// posting

type postResult struct {
	Question cuppa.Question `json:"question"`
	Posted   int            `json:"posted"`  // targets that got it
	Targets  int            `json:"targets"` // targets tried
}

// post picks a question and posts it, like "cuppa post". It answers
// 200 when at least one target got it (check posted vs targets),
// and 502 when none did.
func (s *server) post(w http.ResponseWriter, r *http.Request) {
	if s.a.cfg.Server.Token == "" && !s.allowPost {
		writeError(w, http.StatusForbidden, "post_disabled", "set server.token to allow posting (or start with --insecure-allow-post)")
		return
	}
	if !s.limiter.allow(clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited", fmt.Sprintf("at most %d posts a minute", postsPerMinute))
		return
	}

	target := r.URL.Query().Get("target")
	if target != "" && !slices.ContainsFunc(s.a.cfg.Targets, func(t config.Target) bool { return t.Name == target }) {
		writeError(w, http.StatusNotFound, "not_found", "no target with that name")
		return
	}
	p, err := s.a.newPoster(target, s.a.cfg.Selection)
	if err != nil {
		s.a.log.Error("cannot post", "error", err)
		writeError(w, http.StatusInternalServerError, "config_error", err.Error())
		return
	}

	out, posted, err := s.a.pickAndSend(p)
	if out == nil {
		s.writePick(w, cuppa.Question{}, err)
		return
	}
	if err != nil {
		s.a.log.Error("post did not fully work", "error", err)
	}
	if posted == 0 {
		writeError(w, http.StatusBadGateway, "post_failed", "no target accepted the post")
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: postResult{out.question, posted, len(p.targets)}, Meta: s.meta()})
}

// requireToken lets a request through only with the right bearer
// token, when server.token is set.
func (s *server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		want := s.a.cfg.Server.Token
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		// constant time, so timing can't reveal the token bit by bit
		if want != "" && (!ok || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="cuppa"`)
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or wrong token")
			return
		}
		next(w, r)
	}
}

// clientIP is the address the request came from. X-Forwarded-For is
// ignored, because any client can set it.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimiter allows a few requests per client each minute.
type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	now     func() time.Time
	windows map[string]rateWindow
}

type rateWindow struct {
	start time.Time
	count int
}

func (l *rateLimiter) allow(client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	// forget finished windows now and then, so the map can't keep growing
	if len(l.windows) > 10000 {
		for c, w := range l.windows {
			if now.Sub(w.start) >= time.Minute {
				delete(l.windows, c)
			}
		}
	}

	w := l.windows[client]
	if now.Sub(w.start) >= time.Minute {
		w = rateWindow{start: now}
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	l.windows[client] = w
	return true
}

// service endpoints

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready says whether there are questions to serve.
func (s *server) ready(w http.ResponseWriter, _ *http.Request) {
	if s.all.Len() == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "no questions loaded"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, envelope{Data: map[string]any{
		"version":   Version,
		"commit":    Commit,
		"date":      Date,
		"questions": s.all.Len(),
	}, Meta: s.meta()})
}

func (s *server) openAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(openAPI)
}

// middleware

// cors lets the browser sites in server.cors_origins call the API.
// With none set, browsers on other sites can't read the replies.
func (s *server) cors(next http.Handler) http.Handler {
	origins := s.a.cfg.Server.CORSOrigins
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (slices.Contains(origins, "*") || slices.Contains(origins, origin)) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")

			// a preflight asks first whether the real request is allowed
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// statusWriter remembers the status code, for the request log.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// logRequests logs one line per request and sets headers every
// reply should have.
func (s *server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.a.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "took", time.Since(start).Round(time.Microsecond).String())
	})
}
