package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	requestTimeout = 10 * time.Second
	maxAttempts    = 3
	maxWait        = 30 * time.Second
	maxErrorBody   = 200 // bytes of a failed response to show in the error
)

// firstRetryWait is the wait before the first retry. It doubles
// after each try. Tests make it tiny so they run fast.
var firstRetryWait = time.Second

var httpClient = &http.Client{Timeout: requestTimeout}

// PostJSON sends v as a JSON POST request. See Send.
func PostJSON(ctx context.Context, target string, v any) error {
	body, err := MarshalJSON(v)
	if err != nil {
		return err
	}
	return Send(ctx, http.MethodPost, target, nil, body)
}

// MarshalJSON is json.Marshal without HTML escaping, so characters
// like & and < stay as they are instead of becoming \u0026.
func MarshalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// Send makes an HTTP request and treats any 2xx answer as success.
//
// It tries up to 3 times when the server is busy (429), broken (5xx),
// or can't be reached. The wait between tries doubles each time, and
// a Retry-After header from the server is honored, up to 30 seconds.
//
// Errors never contain the full target URL, because webhook URLs
// carry secrets. Only the scheme and host are shown.
func Send(ctx context.Context, method, target string, headers map[string]string, body []byte) error {
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var retry bool
		var wait time.Duration
		retry, wait, err = sendOnce(ctx, method, target, headers, body)
		if err == nil || !retry || attempt == maxAttempts {
			break
		}

		// no Retry-After from the server: 1x, 2x, 4x the first wait
		if wait <= 0 {
			wait = firstRetryWait << (attempt - 1)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(min(wait, maxWait)):
		}
	}
	return err
}

// sendOnce makes one request. It reports whether a failure is worth
// retrying, and how long the server asked us to wait (0 if it didn't).
func sendOnce(ctx context.Context, method, target string, headers map[string]string, body []byte) (retry bool, wait time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return false, 0, hideURL(err, target)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "cuppa")
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		// network trouble is worth another try, a cancelled run is not
		return ctx.Err() == nil, 0, hideURL(err, target)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body) // lets the connection be reused
		return false, 0, nil
	}

	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	err = fmt.Errorf("%s %s: %s: %s", method, RedactURL(target), resp.Status, strings.TrimSpace(string(snippet)))
	retry = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
	return retry, retryAfter(resp.Header.Get("Retry-After")), err
}

// retryAfter reads a Retry-After header, given either as seconds
// or as a date. It returns 0 when the header is missing or invalid.
func retryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		return time.Until(when)
	}
	return 0
}

// RedactURL keeps only the scheme and host of a URL, so it is safe
// to print: "https://hooks.slack.com/services/T0/B0/xyz" becomes
// "https://hooks.slack.com/...".
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "(hidden url)"
	}
	return u.Scheme + "://" + u.Host + "/..."
}

// hideURL swaps the full URL inside Go's HTTP errors for the
// redacted one, since those errors quote the URL they failed on.
func hideURL(err error, target string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		urlErr.URL = RedactURL(target)
		return urlErr
	}
	// other errors (like a malformed URL) may still quote it
	return errors.New(strings.ReplaceAll(err.Error(), target, RedactURL(target)))
}
