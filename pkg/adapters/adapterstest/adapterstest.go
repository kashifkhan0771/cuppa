// Package adapterstest helps test adapters: it runs a fake chat
// service that records the request it gets.
package adapterstest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/adapters"
	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// Request is one request as the fake service saw it.
type Request struct {
	Method string
	Path   string
	Body   string
	Header http.Header
}

// Server starts a fake chat service that answers 200 OK and keeps the
// last request in the returned Request. It stops when the test ends.
func Server(t *testing.T) (url string, last *Request) {
	t.Helper()
	last = &Request{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*last = Request{Method: r.Method, Path: r.URL.Path, Body: string(body), Header: r.Header}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, last
}

// Message is a sample message with characters that need care in JSON.
var Message = adapters.Message{
	Question: cuppa.Question{ID: "tot-0001", Text: "Coffee or tea?", Options: []string{"Coffee", "Tea"}},
	Rendered: `☕ *Today's watercooler:* Coffee or tea? "quoted" & <b>`,
}
