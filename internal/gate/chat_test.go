package gate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type scriptedModel struct {
	replies []ChatMessage
	index   int
}

func (m *scriptedModel) Next(ctx context.Context, history []ChatMessage) (ChatMessage, error) {
	if m.index >= len(m.replies) {
		return ChatMessage{}, errors.New("unexpected_model_call")
	}
	v := m.replies[m.index]
	m.index++
	return v, nil
}
func toolMessage(name string, p Params) ChatMessage {
	b, _ := json.Marshal(p)
	v := ChatToolCall{}
	v.Function.Name = name
	v.Function.Arguments = b
	return ChatMessage{Role: "assistant", Calls: []ChatToolCall{v}}
}
func TestLocalChatMultiStep(t *testing.T) {
	e, _, i := fixture(t)
	m := &scriptedModel{replies: []ChatMessage{toolMessage("document_read", Params{ResourceID: "doc-1"}), toolMessage("ticket_read", Params{ResourceID: "ticket-1"}), {Role: "assistant", Content: "Read both resources"}}}
	c := NewChatService(e, m)
	s, x := c.Start(context.Background(), i, "read both")
	if x != nil {
		t.Fatal(x)
	}
	if s.Status != "succeeded" || s.Rounds != 3 || len(s.Messages) != 6 {
		t.Fatalf("%+v", s)
	}
	if _, x = os.Stat(filepath.Join(e.Root, s.RunID)); !os.IsNotExist(x) {
		t.Fatal("workspace leaked")
	}
	if _, x = c.Get(context.Background(), Identity{"bob", "beta", "operator"}, s.ID); x == nil {
		t.Fatal("cross tenant chat")
	}
}
func TestLocalChatApprovalResume(t *testing.T) {
	e, _, i := fixture(t)
	m := &scriptedModel{replies: []ChatMessage{toolMessage("ticket_update", Params{"ticket-1", "changed"}), {Role: "assistant", Content: "The approved update completed"}}}
	c := NewChatService(e, m)
	s, x := c.Start(context.Background(), i, "update")
	if x != nil {
		t.Fatal(x)
	}
	if s.Status != "awaiting_approval" || m.index != 1 {
		t.Fatal(s)
	}
	if _, x = c.Resume(context.Background(), i, s.ID); x == nil || x.Error() != "approval_required" {
		t.Fatal(x)
	}
	var digest string
	_ = e.Store.DB.QueryRow(`SELECT digest FROM actions WHERE id=?`, s.Pending).Scan(&digest)
	if _, x = e.Confirm(context.Background(), i, s.Pending, digest, ID()); x != nil {
		t.Fatal(x)
	}
	s, x = c.Resume(context.Background(), i, s.ID)
	if x != nil || s.Status != "succeeded" {
		t.Fatal(s, x)
	}
	if _, x = c.Resume(context.Background(), i, s.ID); x == nil {
		t.Fatal("repeated resume")
	}
	var v int
	_ = e.Store.DB.QueryRow(`SELECT version FROM resources WHERE id='ticket-1' AND tenant='acme'`).Scan(&v)
	if v != 2 {
		t.Fatal(v)
	}
}
func TestLocalChatDenialAndLimits(t *testing.T) {
	for _, name := range []string{"foreign", "loop", "injected"} {
		t.Run(name, func(t *testing.T) {
			e, _, i := fixture(t)
			var replies []ChatMessage
			switch name {
			case "foreign":
				replies = []ChatMessage{toolMessage("document_read", Params{ResourceID: "doc-2"})}
			case "loop":
				for n := 0; n < 9; n++ {
					replies = append(replies, toolMessage("document_read", Params{ResourceID: "doc-1"}))
				}
			case "injected":
				msg := toolMessage("document_read", Params{ResourceID: "doc-1"})
				msg.Calls[0].Function.Arguments = json.RawMessage(`{"resource_id":"doc-1","tenant_id":"beta"}`)
				replies = []ChatMessage{msg}
			}
			c := NewChatService(e, &scriptedModel{replies: replies})
			if _, x := c.Start(context.Background(), i, "test"); x == nil {
				t.Fatal("unsafe plan accepted")
			}
			var n int
			_ = e.Store.DB.QueryRow(`SELECT count(*) FROM runs WHERE status='running'`).Scan(&n)
			if n != 0 {
				t.Fatal("failed run leaked")
			}
		})
	}
}

type blockedModel struct {
	started chan struct{}
	calls   atomic.Int32
}

func (m *blockedModel) Next(ctx context.Context, _ []ChatMessage) (ChatMessage, error) {
	m.calls.Add(1)
	close(m.started)
	<-ctx.Done()
	return ChatMessage{}, ctx.Err()
}
func TestLocalChatRevocationCancelsInference(t *testing.T) {
	e, _, i := fixture(t)
	m := &blockedModel{started: make(chan struct{})}
	c := NewChatService(e, m)
	done := make(chan error, 1)
	go func() { _, x := c.Start(context.Background(), i, "read"); done <- x }()
	<-m.started
	if x := e.Revoke(context.Background(), i); x != nil {
		t.Fatal(x)
	}
	select {
	case x := <-done:
		if x == nil {
			t.Fatal("revoked response returned")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("inference not cancelled")
	}
	if m.calls.Load() != 1 {
		t.Fatal("retried")
	}
}
func TestLocalChatRestart(t *testing.T) {
	e, _, i := fixture(t)
	c := NewChatService(e, &scriptedModel{replies: []ChatMessage{toolMessage("ticket_update", Params{"ticket-1", "new"})}})
	s, x := c.Start(context.Background(), i, "update")
	if x != nil {
		t.Fatal(x)
	}
	if _, x = e.Store.DB.Exec(chatMigration); x != nil {
		t.Fatal(x)
	}
	if _, x = c.Resume(context.Background(), i, s.ID); x == nil {
		t.Fatal("resumed interrupted chat")
	}
}
func TestLocalModelConfiguration(t *testing.T) {
	for _, m := range []string{"", "qwen3-cloud", "../model", "qwen\n3"} {
		if _, x := NewOllama(m); x == nil {
			t.Fatal("invalid model", m)
		}
	}
	o, x := NewOllama("qwen3:1.7b")
	if x != nil {
		t.Fatal(x)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, x = o.Next(ctx, []ChatMessage{{Role: "user", Content: "hello"}}); x == nil {
		t.Fatal("cancel ignored")
	}
}

func TestCancelledChatOverviewAndPagination(t *testing.T) {
	e, a, i := fixture(t)
	c := NewChatService(e, &scriptedModel{replies: []ChatMessage{toolMessage("ticket_update", Params{"ticket-1", "new"})}})
	chat, err := c.Start(context.Background(), i, "update")
	if err != nil {
		t.Fatal(err)
	}
	if err = e.End(context.Background(), i, chat.RunID, "cancelled"); err != nil {
		t.Fatal(err)
	}
	server := NewServer(e, a, nil)
	for _, offset := range []string{"abc", "-1", "100001"} {
		if _, err = server.overview(context.Background(), i, httptest.NewRequest("GET", "/api/overview?offset="+offset, nil)); err == nil {
			t.Fatal("bad offset accepted", offset)
		}
	}
	raw, err := server.overview(context.Background(), i, httptest.NewRequest("GET", "/api/overview", nil))
	if err != nil {
		t.Fatal(err)
	}
	overview := raw.(map[string]any)
	for _, key := range []string{"chats", "approvals"} {
		rows := overview[key].([]map[string]any)
		if len(rows) != 1 || rows[0]["status"] != "cancelled" {
			t.Fatal(key, rows)
		}
	}
	fetched, err := c.Get(context.Background(), i, chat.ID)
	if err != nil || fetched.Status != "cancelled" {
		t.Fatal(fetched, err)
	}
}

func TestChatFinalizationRollback(t *testing.T) {
	e, _, i := fixture(t)
	// Simulate a storage failure only at the final answer update.
	_, err := e.Store.DB.Exec(`CREATE TRIGGER reject_answer BEFORE UPDATE ON chats WHEN NEW.status='succeeded' BEGIN SELECT RAISE(ABORT,'simulated persistence failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	c := NewChatService(e, &scriptedModel{replies: []ChatMessage{{Role: "assistant", Content: "answer"}}})
	if _, err = c.Start(context.Background(), i, "hello"); err == nil {
		t.Fatal("failed persistence returned success")
	}
	var chatStatus, runStatus string
	if err = e.Store.DB.QueryRow(`SELECT c.status,r.status FROM chats c JOIN runs r ON c.run_id=r.id`).Scan(&chatStatus, &runStatus); err != nil {
		t.Fatal(err)
	}
	if chatStatus != "failed" || runStatus != "failed" {
		t.Fatal(chatStatus, runStatus)
	}
	var successes int
	if err = e.Store.DB.QueryRow(`SELECT count(*) FROM audit WHERE action='run.end' AND outcome='succeeded'`).Scan(&successes); err != nil {
		t.Fatal(err)
	}
	if successes != 0 {
		t.Fatal("false success audit", successes)
	}
}

func TestChatOversizedFailureStillTerminates(t *testing.T) {
	e, _, i := fixture(t)
	c := NewChatService(e, &scriptedModel{replies: []ChatMessage{toolMessage("ticket_update", Params{"ticket-1", "new"})}})
	s, err := c.Start(context.Background(), i, "update")
	if err != nil {
		t.Fatal(err)
	}
	s.Messages = append(s.Messages, ChatMessage{Role: "tool", Content: strings.Repeat("x", 65537)})
	c.fail(i, s, "failed")
	var status, pending string
	if err = e.Store.DB.QueryRow(`SELECT status,pending FROM chats WHERE id=?`, s.ID).Scan(&status, &pending); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || pending != "" {
		t.Fatal(status, pending)
	}
}
