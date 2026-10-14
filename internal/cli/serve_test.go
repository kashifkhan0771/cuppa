package cli

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
)

// testServer starts the API on the cuppa.yaml in the current folder.
func testServer(t *testing.T, allowPost bool) *httptest.Server {
	t.Helper()
	a := &app{ctx: context.Background(), stdout: io.Discard, stderr: io.Discard, now: timeNow, sleep: sleep,
		log: slog.New(slog.DiscardHandler), configPath: "cuppa.yaml"}
	if err := a.loadConfig(); err != nil {
		t.Fatal(err)
	}
	s, err := a.newServer(allowPost)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s.handler())
	t.Cleanup(srv.Close)
	return srv
}

type reply struct {
	status int
	header http.Header
	body   map[string]any
}

// call makes a request with optional headers given as "Name: value".
func call(t *testing.T, method, url string, headers ...string) reply {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range headers {
		name, value, _ := strings.Cut(h, ": ")
		req.Header.Set(name, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	r := reply{status: resp.StatusCode, header: resp.Header}
	_ = json.NewDecoder(resp.Body).Decode(&r.body)
	return r
}

// errorCode is the error.code of a reply, or "".
func (r reply) errorCode() string {
	e, _ := r.body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

// field reads data.<name> from a reply.
func (r reply) field(name string) any {
	data, _ := r.body["data"].(map[string]any)
	return data[name]
}

func TestServeQuestions(t *testing.T) {
	isolate(t)
	t.Setenv("HOOK_ONE", "http://x")
	postConfig(t, "daily", "")
	url := testServer(t, false).URL

	r := call(t, "GET", url+"/v1/questions/random")
	if r.status != 200 || r.field("category") != "team" || r.header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("random: %+v", r)
	}
	if m, _ := r.body["meta"].(map[string]any); m["api_version"] != "v1" || m["dataset_version"] == "" {
		t.Errorf("meta: %v", r.body["meta"])
	}

	// the same seed gives the same question on the same day
	first := call(t, "GET", url+"/v1/questions/today").field("id")
	if second := call(t, "GET", url+"/v1/questions/today").field("id"); first == nil || first != second {
		t.Errorf("today: %v then %v", first, second)
	}

	if r := call(t, "GET", url+"/v1/questions/team-0001"); r.status != 200 || r.field("id") != "team-0001" {
		t.Errorf("by id: %+v", r)
	}
	if r := call(t, "GET", url+"/v1/questions/nope-0001"); r.status != 404 || r.errorCode() != "not_found" {
		t.Errorf("unknown id: %+v", r)
	}

	// filters narrow the config's selection
	if r := call(t, "GET", url+"/v1/questions/random?category=icebreakers&sfw=casual&type=open&locale=en"); r.status != 200 || r.field("category") != "icebreakers" {
		t.Errorf("filters: %+v", r)
	}
	if r := call(t, "GET", url+"/v1/questions/random?category=nope"); r.status != 404 || r.errorCode() != "no_match" {
		t.Errorf("no match: %+v", r)
	}
	if r := call(t, "GET", url+"/v1/questions/today?sfw=wild"); r.status != 400 || r.errorCode() != "bad_request" {
		t.Errorf("bad sfw: %+v", r)
	}
}

func TestServeList(t *testing.T) {
	isolate(t)
	t.Setenv("HOOK_ONE", "http://x")
	postConfig(t, "daily", "")
	url := testServer(t, false).URL

	r := call(t, "GET", url+"/v1/questions?limit=1&offset=1")
	m, _ := r.body["meta"].(map[string]any)
	if data, _ := r.body["data"].([]any); r.status != 200 || len(data) != 1 || m["total"] != 2.0 || m["offset"] != 1.0 {
		t.Errorf("page: %+v", r)
	}
	// past the end is an empty list, not null
	if r := call(t, "GET", url+"/v1/questions?offset=50"); r.status != 200 || r.body["data"] == nil {
		t.Errorf("past the end: %+v", r)
	}
	for _, query := range []string{"limit=0", "limit=1001", "limit=x", "offset=-1", "sfw=wild"} {
		if r := call(t, "GET", url+"/v1/questions?"+query); r.status != 400 {
			t.Errorf("%s: %+v", query, r)
		}
	}

	if r := call(t, "GET", url+"/v1/categories"); r.status != 200 || len(r.body["data"].([]any)) < 6 {
		t.Errorf("categories: %+v", r)
	}
}

func TestServeServiceEndpoints(t *testing.T) {
	isolate(t)
	t.Setenv("HOOK_ONE", "http://x")
	postConfig(t, "daily", "")
	url := testServer(t, false).URL

	for _, path := range []string{"/healthz", "/readyz"} {
		if r := call(t, "GET", url+path); r.status != 200 || r.body["status"] != "ok" {
			t.Errorf("%s: %+v", path, r)
		}
	}
	if r := call(t, "GET", url+"/version"); r.status != 200 || r.field("questions") == nil {
		t.Errorf("version: %+v", r)
	}
	if r := call(t, "GET", url+"/openapi.json"); r.status != 200 || r.body["openapi"] != "3.1.0" {
		t.Errorf("openapi: %+v", r)
	}
	if r := call(t, "DELETE", url+"/healthz"); r.status != http.StatusMethodNotAllowed {
		t.Errorf("wrong method: %+v", r)
	}
}

func TestServeToken(t *testing.T) {
	isolate(t)
	t.Setenv("HOOK_ONE", "http://x")
	postConfig(t, "daily", "server:\n  token: ${API_TOKEN}\n")
	t.Setenv("API_TOKEN", "s3cret")
	url := testServer(t, false).URL + "/v1/questions/next"

	if r := call(t, "GET", url); r.status != 401 || r.errorCode() != "unauthorized" || r.header.Get("WWW-Authenticate") == "" {
		t.Errorf("no token: %+v", r)
	}
	if r := call(t, "GET", url, "Authorization: Bearer wrong"); r.status != 401 {
		t.Errorf("wrong token: %+v", r)
	}
	if r := call(t, "GET", url, "Authorization: Bearer s3cret"); r.status != 200 {
		t.Errorf("right token: %+v", r)
	}
	// reading questions never needs the token
	if r := call(t, "GET", url[:strings.LastIndex(url, "/")]+"/random"); r.status != 200 {
		t.Errorf("random: %+v", r)
	}
}

func TestServePost(t *testing.T) {
	isolate(t)
	chat := newChat(t, http.StatusOK)
	t.Setenv("HOOK_ONE", chat.url)
	postConfig(t, "no-repeat", "")

	// no token: posting is off unless allowed on purpose
	if r := call(t, "POST", testServer(t, false).URL+"/v1/post"); r.status != 403 || r.errorCode() != "post_disabled" {
		t.Errorf("no token: %+v", r)
	}

	url := testServer(t, true).URL + "/v1/post"
	r := call(t, "POST", url)
	if r.status != 200 || r.field("posted") != 1.0 || r.field("targets") != 1.0 || len(chat.received()) != 1 {
		t.Errorf("post: %+v", r)
	}
	if r := call(t, "POST", url+"?target=nope"); r.status != 404 || r.errorCode() != "not_found" {
		t.Errorf("unknown target: %+v", r)
	}

	// 10 posts a minute per client; the clock is fixed in tests
	for range postsPerMinute - 2 {
		call(t, "POST", url+"?target=one")
	}
	if r := call(t, "POST", url); r.status != 429 || r.errorCode() != "rate_limited" || r.header.Get("Retry-After") != "60" {
		t.Errorf("rate limit: %+v", r)
	}
}

func TestServePostFailures(t *testing.T) {
	isolate(t)
	chat := newChat(t, http.StatusNotFound)
	t.Setenv("HOOK_ONE", chat.url)
	postConfig(t, "daily", "")
	url := testServer(t, true).URL + "/v1/post"

	if r := call(t, "POST", url); r.status != 502 || r.errorCode() != "post_failed" {
		t.Errorf("all targets failed: %+v", r)
	}

	// a target that can't be built is a config problem
	t.Setenv("HOOK_ONE", "")
	if r := call(t, "POST", testServer(t, true).URL+"/v1/post"); r.status != 500 || r.errorCode() != "config_error" {
		t.Errorf("broken target: %+v", r)
	}
}

func TestServeCORS(t *testing.T) {
	isolate(t)
	t.Setenv("HOOK_ONE", "http://x")
	postConfig(t, "daily", "server:\n  cors_origins: [https://intranet.example]\n")
	url := testServer(t, false).URL + "/v1/questions/random"

	r := call(t, "GET", url, "Origin: https://intranet.example")
	if r.header.Get("Access-Control-Allow-Origin") != "https://intranet.example" {
		t.Errorf("allowed origin: %v", r.header)
	}
	r = call(t, "OPTIONS", url, "Origin: https://intranet.example", "Access-Control-Request-Method: GET")
	if r.status != 204 || !strings.Contains(r.header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Errorf("preflight: %+v", r)
	}
	if r := call(t, "GET", url, "Origin: https://evil.example"); r.header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("other origin allowed: %v", r.header)
	}
}

// Many callers at once must never get the same question.
func TestServeNextConcurrent(t *testing.T) {
	isolate(t)
	writeFile(t, "cuppa.yaml", "selection:\n  mode: no-repeat\nhistory:\n  path: h.json\n")
	url := testServer(t, false).URL + "/v1/questions/next"

	const callers = 20
	ids := make([]string, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Go(func() {
			ids[i], _ = call(t, "GET", url).field("id").(string)
		})
	}
	wg.Wait()

	slices.Sort(ids)
	if len(slices.Compact(ids)) != callers || ids[0] == "" {
		t.Errorf("got repeats or errors: %v", ids)
	}
}

// openapi.json must list exactly the routes the server has.
func TestOpenAPIMatchesRoutes(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(openAPI, &spec); err != nil {
		t.Fatal(err)
	}
	var documented, served []string
	for path, methods := range spec.Paths {
		for method := range methods {
			documented = append(documented, strings.ToUpper(method)+" "+path)
		}
	}
	for _, r := range (&server{}).routes() {
		served = append(served, r.pattern)
	}
	slices.Sort(documented)
	slices.Sort(served)
	if !slices.Equal(documented, served) {
		t.Errorf("openapi.json paths:\n%v\nserver routes:\n%v", documented, served)
	}
}

func TestRunServe(t *testing.T) {
	isolate(t)
	t.Setenv("HOOK_ONE", "http://x")
	postConfig(t, "daily", "schedule:\n  cron: \"0 10 * * *\"\n")

	// an already stopped cuppa starts, then shuts down cleanly
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stderr strings.Builder
	code := Run(ctx, []string{"serve", "--addr", "127.0.0.1:0", "--with-scheduler"}, strings.NewReader(""), io.Discard, &stderr)
	if code != exitOK || !strings.Contains(stderr.String(), "listening") || !strings.Contains(stderr.String(), "stopped") {
		t.Errorf("serve: %d %s", code, stderr.String())
	}

	// a port in use is an error
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	if r := run(t, "", "serve", "--addr", busy.Addr().String()); r.code != exitFailed {
		t.Errorf("busy port: %+v", r)
	}

	// --with-scheduler needs a cron line
	postConfig(t, "daily", "")
	if r := run(t, "", "serve", "--with-scheduler"); r.code != exitConfig || !strings.Contains(r.stderr, "schedule.cron") {
		t.Errorf("no cron: %+v", r)
	}
}

func TestListenAddr(t *testing.T) {
	if got := listenAddr("", ""); got != "127.0.0.1:8080" {
		t.Errorf("empty: %s", got)
	}
	if got := listenAddr("", "0.0.0.0:9"); got != "0.0.0.0:9" {
		t.Errorf("config: %s", got)
	}
	if got := listenAddr(":1", "0.0.0.0:9"); got != ":1" {
		t.Errorf("flag wins: %s", got)
	}
}

func TestIsLoopback(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:8080": true, "localhost:8080": true, "[::1]:8080": true,
		"0.0.0.0:8080": false, ":8080": false, "192.168.1.5:80": false, "nonsense": false,
	} {
		if got := isLoopback(addr); got != want {
			t.Errorf("%s: got %v", addr, got)
		}
	}
}

// The /v1 replies are a promise: these fields must stay. New fields may be
// added, so the test only checks that the listed ones are present.
func TestServeV1FieldsStay(t *testing.T) {
	isolate(t)
	t.Setenv("HOOK_ONE", "http://x")
	postConfig(t, "daily", "")
	url := testServer(t, false).URL

	has := func(what string, got map[string]any, keys ...string) {
		t.Helper()
		for _, key := range keys {
			if _, ok := got[key]; !ok {
				t.Errorf("%s lost the %q field: %v", what, key, got)
			}
		}
	}
	asMap := func(v any) map[string]any {
		m, _ := v.(map[string]any)
		return m
	}

	question := []string{"id", "category", "text", "type", "sfw", "locale", "added"}
	r := call(t, "GET", url+"/v1/questions/today")
	has("question", asMap(r.body["data"]), question...)
	has("meta", asMap(r.body["meta"]), "dataset_version", "api_version")

	r = call(t, "GET", url+"/v1/questions?limit=1")
	has("list question", asMap(r.body["data"].([]any)[0]), question...)
	has("list meta", asMap(r.body["meta"]), "dataset_version", "api_version", "total", "limit")

	r = call(t, "GET", url+"/v1/categories")
	has("category", asMap(r.body["data"].([]any)[0]), "id", "name", "count")

	r = call(t, "GET", url+"/version")
	has("version", asMap(r.body["data"]), "version", "commit", "date", "questions")

	r = call(t, "GET", url+"/v1/questions/nope-0001")
	has("error", asMap(r.body["error"]), "code", "message")
}
