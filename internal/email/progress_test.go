package email_test

import (
	"context"
	"testing"

	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/email"
	"github.com/ntub/domjudge-tool/internal/model"
)

// recordingReporter satisfies progress.Reporter for tests by counting
// Tick invocations. The interface is satisfied structurally so this
// file does not need to import the progress package.
type recordingReporter struct {
	starts   int
	ticks    int
	finishes int
	total    int
	label    string
}

func (r *recordingReporter) Start(total int, label string) {
	r.starts++
	r.total = total
	r.label = label
}
func (r *recordingReporter) Tick()   { r.ticks++ }
func (r *recordingReporter) Finish() { r.finishes++ }

// TestSendUserAccounts_Reporter_TicksOncePerRow asserts that a Reporter
// passed via SendOptions sees exactly one Tick per successfully-sent
// user, plus one Start and one Finish around the loop.
func TestSendUserAccounts_Reporter_TicksOncePerRow(t *testing.T) {
	stub := newStubSender()
	tmpl := &email.Template{Subject: "hi", Body: "hey", BodyHTML: false}
	users := []model.CreateUser{
		{Username: "alice", Name: "Alice"},
		{Username: "bob", Name: "Bob"},
		{Username: "carol", Name: "Carol"},
	}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	rep := &recordingReporter{}
	opts := email.SendOptions{FromEmail: "noreply@example.com", Reporter: rep}

	if err := email.SendUserAccounts(context.Background(), stub, tmpl, users, cfg, opts); err != nil {
		t.Fatalf("SendUserAccounts: %v", err)
	}
	if rep.starts != 1 {
		t.Errorf("starts = %d, want 1", rep.starts)
	}
	if rep.total != len(users) {
		t.Errorf("Start total = %d, want %d", rep.total, len(users))
	}
	if rep.ticks != len(users) {
		t.Errorf("ticks = %d, want %d (one per user)", rep.ticks, len(users))
	}
	if rep.finishes != 1 {
		t.Errorf("finishes = %d, want 1", rep.finishes)
	}
}

// TestSendUserAccounts_Reporter_NilUsesNop asserts that omitting
// SendOptions.Reporter (nil) does not panic and behaves as a silent
// no-op — the library MUST default nil to progress.Nop().
func TestSendUserAccounts_Reporter_NilUsesNop(t *testing.T) {
	stub := newStubSender()
	tmpl := &email.Template{Subject: "hi", Body: "hey", BodyHTML: false}
	users := []model.CreateUser{
		{Username: "alice", Name: "Alice"},
	}
	cfg := &config.DomServerClient{Host: "judge.example.com"}
	opts := email.SendOptions{FromEmail: "noreply@example.com"}

	if err := email.SendUserAccounts(context.Background(), stub, tmpl, users, cfg, opts); err != nil {
		t.Fatalf("SendUserAccounts(nil reporter): %v", err)
	}
	if len(stub.calls) != 1 {
		t.Errorf("stub send count = %d, want 1", len(stub.calls))
	}
}
