package teams_test

import (
	"context"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/adapters"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/adapterstest"
	"github.com/kashifkhan0771/cuppa/pkg/adapters/teams"
)

func TestPayload(t *testing.T) {
	url, got := adapterstest.Server(t)
	message := adapters.Message{Rendered: "☕ **Today:** Coffee or tea?\n1️⃣ Coffee\n\n2️⃣ \"Tea\" & <b>\n"}
	if err := teams.New(url+"/workflows/x").Post(context.Background(), message); err != nil {
		t.Fatal(err)
	}

	// one text block per line, blank lines dropped
	want := `{"type":"message","attachments":[{"contentType":"application/vnd.microsoft.card.adaptive","content":{"$schema":"http://adaptivecards.io/schemas/adaptive-card.json","type":"AdaptiveCard","version":"1.2","body":[` +
		`{"type":"TextBlock","text":"☕ **Today:** Coffee or tea?","wrap":true},` +
		`{"type":"TextBlock","text":"1️⃣ Coffee","wrap":true},` +
		`{"type":"TextBlock","text":"2️⃣ \"Tea\" & <b>","wrap":true}]}}]}`
	if got.Method != "POST" || got.Path != "/workflows/x" || got.Body != want {
		t.Errorf("got %s %s %s\nwant body %s", got.Method, got.Path, got.Body, want)
	}
}
