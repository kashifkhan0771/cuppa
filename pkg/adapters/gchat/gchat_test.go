package gchat_test

import (
	"context"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/adapters/adapterstest"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/gchat"
)

func TestPayload(t *testing.T) {
	url, got := adapterstest.Server(t)
	if err := gchat.New(url+"/v1/spaces/x/messages").Post(context.Background(), adapterstest.Message); err != nil {
		t.Fatal(err)
	}

	want := `{"text":"☕ *Today's watercooler:* Coffee or tea? \"quoted\" & <b>"}`
	if got.Method != "POST" || got.Path != "/v1/spaces/x/messages" || got.Body != want {
		t.Errorf("got %s %s %s\nwant body %s", got.Method, got.Path, got.Body, want)
	}
}
