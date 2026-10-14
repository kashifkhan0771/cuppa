package adapters

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// retries wait a millisecond instead of a second
	firstRetryWait = time.Millisecond
	os.Exit(m.Run())
}

// server answers with the given status codes in order, repeating the
// last one, and counts the requests it got.
func server(t *testing.T, statuses ...int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1))
		status := statuses[min(n, len(statuses))-1]
		w.WriteHeader(status)
		_, _ = io.WriteString(w, "server says "+http.StatusText(status))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestSendSuccess(t *testing.T) {
	var got *http.Request
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got, body = r, string(raw)
	}))
	defer srv.Close()

	err := Send(context.Background(), http.MethodPut, srv.URL+"/hook", map[string]string{"X-Token": "abc"}, []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPut || got.URL.Path != "/hook" || body != `{"a":1}` {
		t.Errorf("got %s %s %s", got.Method, got.URL.Path, body)
	}
	if got.Header.Get("Content-Type") != "application/json" || got.Header.Get("X-Token") != "abc" || got.Header.Get("User-Agent") != "cuppa" {
		t.Errorf("headers = %v", got.Header)
	}
}

func TestSendRetries(t *testing.T) {
	tests := []struct {
		name      string
		statuses  []int
		wantCalls int32
		wantErr   string
	}{
		{"5xx then ok", []int{500, 503, 200}, 3, ""},
		{"429 then ok", []int{429, 200}, 2, ""},
		{"gives up after 3 tries", []int{502}, 3, "502 Bad Gateway: server says Bad Gateway"},
		{"4xx is not retried", []int{404}, 1, "404 Not Found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, calls := server(t, tt.statuses...)
			err := Send(context.Background(), http.MethodPost, srv.URL, nil, nil)

			if calls.Load() != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls.Load(), tt.wantCalls)
			}
			if tt.wantErr == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSendHidesURLInErrors(t *testing.T) {
	secret := "/services/T000/B000/very-secret"

	t.Run("bad status", func(t *testing.T) {
		srv, _ := server(t, 403)
		err := Send(context.Background(), http.MethodPost, srv.URL+secret, nil, nil)
		if err == nil || strings.Contains(err.Error(), "very-secret") {
			t.Fatalf("secret leaked or no error: %v", err)
		}
	})

	t.Run("network error", func(t *testing.T) {
		// a closed server refuses the connection
		srv, _ := server(t, 200)
		srv.Close()
		err := Send(context.Background(), http.MethodPost, srv.URL+secret, nil, nil)
		if err == nil || strings.Contains(err.Error(), "very-secret") || !strings.Contains(err.Error(), "/...") {
			t.Fatalf("secret leaked or no error: %v", err)
		}
	})

	t.Run("malformed url", func(t *testing.T) {
		err := Send(context.Background(), http.MethodPost, "http://bad host"+secret, nil, nil)
		if err == nil || strings.Contains(err.Error(), "very-secret") {
			t.Fatalf("secret leaked or no error: %v", err)
		}
	})
}

func TestSendStopsWhenCancelled(t *testing.T) {
	srv, calls := server(t, 500)
	ctx, cancel := context.WithCancel(context.Background())
	firstRetryWait = time.Hour // would hang if the cancel were ignored
	defer func() { firstRetryWait = time.Millisecond }()

	go func() {
		for calls.Load() == 0 {
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	if err := Send(ctx, http.MethodPost, srv.URL, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestPostJSON(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
	}))
	defer srv.Close()

	if err := PostJSON(context.Background(), srv.URL, map[string]string{"text": "hi"}); err != nil {
		t.Fatal(err)
	}
	if body != `{"text":"hi"}` {
		t.Errorf("body = %s", body)
	}
	if err := PostJSON(context.Background(), srv.URL, func() {}); err == nil {
		t.Error("want error for a value JSON can't encode")
	}
}

func TestRetryAfter(t *testing.T) {
	tests := []struct {
		value string
		want  time.Duration
	}{
		{"", 0},
		{"5", 5 * time.Second},
		{"0", 0},
		{"soon", 0},
		{"Mon, 02 Jan 2006 15:04:05 GMT", 0}, // a date in the past means no wait
	}
	for _, tt := range tests {
		if got := retryAfter(tt.value); (tt.want == 0 && got > 0) || (tt.want != 0 && got != tt.want) {
			t.Errorf("retryAfter(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
	future := time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)
	if got := retryAfter(future); got <= 0 || got > time.Minute {
		t.Errorf("retryAfter(future date) = %v", got)
	}
}

func TestRedactURL(t *testing.T) {
	tests := map[string]string{
		"https://hooks.slack.com/services/T0/B0/xyz":            "https://hooks.slack.com/...",
		"https://chat.googleapis.com/v1/spaces/A?key=k&token=t": "https://chat.googleapis.com/...",
		"not a url":       "(hidden url)",
		"://broken":       "(hidden url)",
		"http://bad host": "(hidden url)",
	}
	for in, want := range tests {
		if got := RedactURL(in); got != want {
			t.Errorf("RedactURL(%q) = %q, want %q", in, got, want)
		}
	}
}
