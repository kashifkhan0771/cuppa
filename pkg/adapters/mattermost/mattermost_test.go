package mattermost_test

import (
	"context"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/adapters/adapterstest"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/mattermost"
)

func TestPayload(t *testing.T) {
	url, got := adapterstest.Server(t)
	if err := mattermost.New(url+"/hooks/x").Post(context.Background(), adapterstest.Message); err != nil {
		t.Fatal(err)
	}

	want := `{"text":"☕ *Today's watercooler:* Coffee or tea? \"quoted\" & <b>"}`
	if got.Method != "POST" || got.Path != "/hooks/x" || got.Body != want {
		t.Errorf("got %s %s %s\nwant body %s", got.Method, got.Path, got.Body, want)
	}
}
