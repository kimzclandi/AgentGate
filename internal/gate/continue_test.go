package gate

import (
	"context"
	"strings"
	"testing"
)

type historyModel struct{ seen []ChatMessage }

func (m *historyModel) Next(_ context.Context, h []ChatMessage) (ChatMessage, error) {
	m.seen = h
	return ChatMessage{Role: "assistant", Content: "recorded"}, nil
}
func TestContinueIsolationAndContext(t *testing.T) {
	e, _, i := fixture(t)
	m := &historyModel{}
	c := NewChatService(e, m)
	ctx := context.Background()
	first, err := c.Start(ctx, i, "Remember ticket-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Continue(ctx, i, first.ID, "Read it now")
	if err != nil {
		t.Fatal(err)
	}
	if second.ParentID != first.ID || first.RunID == second.RunID || first.ID == second.ID || len(m.seen) != 4 || m.seen[1].Content != "Remember ticket-1" {
		t.Fatal(second, m.seen)
	}
	loaded, err := c.Get(ctx, i, second.ID)
	if err != nil || loaded.ParentID != first.ID {
		t.Fatal("lost persisted parent", loaded, err)
	}
	if _, err = c.Continue(ctx, Identity{"bob", "beta", "operator"}, first.ID, "read"); err == nil {
		t.Fatal("cross-tenant access")
	}
	if _, err = c.Continue(ctx, i, first.ID, strings.Repeat("x", 4097)); err == nil {
		t.Fatal("unbounded task")
	}
}
func TestContinueCannotBypassApproval(t *testing.T) {
	e, _, i := fixture(t)
	m := &scriptedModel{replies: []ChatMessage{toolMessage("ticket_update", Params{"ticket-1", "changed"})}}
	c := NewChatService(e, m)
	s, err := c.Start(context.Background(), i, "update")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Continue(context.Background(), i, s.ID, "skip approval"); err == nil || err.Error() != "chat_not_continuable" {
		t.Fatal(err)
	}
}
