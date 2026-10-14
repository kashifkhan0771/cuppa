package webex

import (
	"context"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/adapters/adapterstest"
)

func TestPayload(t *testing.T) {
	url, got := adapterstest.Server(t)
	old := messagesURL
	messagesURL = url + "/v1/messages"
	t.Cleanup(func() { messagesURL = old })

	err := New(Options{Token: "secret-token", RoomID: "room-1"}).Post(context.Background(), adapterstest.Message)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"roomId":"room-1","markdown":"☕ *Today's watercooler:* Coffee or tea? \"quoted\" & <b>"}`
	if got.Method != "POST" || got.Path != "/v1/messages" || got.Body != want {
		t.Errorf("got %s %s %s\nwant body %s", got.Method, got.Path, got.Body, want)
	}
	if auth := got.Header.Get("Authorization"); auth != "Bearer secret-token" {
		t.Errorf("Authorization = %q", auth)
	}
}
