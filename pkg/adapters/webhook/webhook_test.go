package webhook_test

import (
	"context"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/adapters/adapterstest"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/webhook"
)

func TestPayloads(t *testing.T) {
	tests := []struct {
		name       string
		options    webhook.Options
		wantMethod string
		wantBody   string
	}{
		{
			name:       "defaults",
			options:    webhook.Options{},
			wantMethod: "POST",
			wantBody:   `{"text": "☕ *Today's watercooler:* Coffee or tea? \"quoted\" & <b>"}`,
		},
		{
			name: "custom method and body",
			options: webhook.Options{
				Method: "PUT",
				Body:   `{"q": {{ json .Question.ID }}, "options": {{ json .Question.Options }}}`,
			},
			wantMethod: "PUT",
			wantBody:   `{"q": "tot-0001", "options": ["Coffee","Tea"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, got := adapterstest.Server(t)
			tt.options.URL = url + "/hook"
			adapter, err := webhook.New(tt.options)
			if err != nil {
				t.Fatal(err)
			}
			if err := adapter.Post(context.Background(), adapterstest.Message); err != nil {
				t.Fatal(err)
			}
			if got.Method != tt.wantMethod || got.Path != "/hook" || got.Body != tt.wantBody {
				t.Errorf("got %s %s %s\nwant body %s", got.Method, got.Path, got.Body, tt.wantBody)
			}
		})
	}
}

func TestHeaders(t *testing.T) {
	url, got := adapterstest.Server(t)
	adapter, err := webhook.New(webhook.Options{URL: url, Headers: map[string]string{"Authorization": "Bearer x", "Content-Type": "text/plain"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Post(context.Background(), adapterstest.Message); err != nil {
		t.Fatal(err)
	}
	if got.Header.Get("Authorization") != "Bearer x" || got.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("headers = %v", got.Header)
	}
}

func TestTemplateErrors(t *testing.T) {
	if _, err := webhook.New(webhook.Options{URL: "http://x", Body: "{{ .Broken"}); err == nil {
		t.Error("want error for a template that doesn't parse")
	}

	adapter, err := webhook.New(webhook.Options{URL: "http://x", Body: "{{ .NoSuchField }}"})
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Post(context.Background(), adapterstest.Message); err == nil {
		t.Error("want error for a template that fails to run")
	}
}
