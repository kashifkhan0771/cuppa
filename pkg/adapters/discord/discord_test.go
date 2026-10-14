package discord_test

import (
	"context"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/adapters/adapterstest"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/discord"
)

func TestPayload(t *testing.T) {
	url, got := adapterstest.Server(t)
	if err := discord.New(url+"/api/webhooks/1/x").Post(context.Background(), adapterstest.Message); err != nil {
		t.Fatal(err)
	}

	// "parse": [] must be an empty list, not null, to turn off pings
	want := `{"content":"☕ *Today's watercooler:* Coffee or tea? \"quoted\" & <b>","allowed_mentions":{"parse":[]}}`
	if got.Method != "POST" || got.Path != "/api/webhooks/1/x" || got.Body != want {
		t.Errorf("got %s %s %s\nwant body %s", got.Method, got.Path, got.Body, want)
	}
}
