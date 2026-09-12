package gate

import (
	"context"
	"errors"
	"strings"
)

// MockTask is deterministic. Only the authenticated task selects a registered tool;
// retrieved documents and tool results are never parsed as instructions.
func (e *Engine) MockTask(ctx context.Context, i Identity, task, request string) (any, error) {
	if len(task) > 4096 {
		return nil, errors.New("invalid_input")
	}
	parts := strings.SplitN(task, " ", 3)
	if len(parts) < 2 {
		return nil, errors.New("invalid_task")
	}
	var tool, scope string
	var p Params
	p.ResourceID = parts[1]
	switch parts[0] {
	case "read-doc":
		tool = "document.read"
		scope = "document:read"
	case "read-ticket":
		tool = "ticket.read"
		scope = "ticket:read"
	case "update-ticket":
		if len(parts) != 3 {
			return nil, errors.New("invalid_task")
		}
		tool = "ticket.update"
		scope = "ticket:write"
		p.Body = parts[2]
	default:
		return nil, errors.New("invalid_task")
	}
	r, er := e.CreateRun(ctx, i, "assistant", []string{scope}, 120)
	if er != nil {
		return nil, er
	}
	if _, er = e.Store.DB.ExecContext(ctx, `UPDATE runs SET auto_finish=1 WHERE id=?`, r.ID); er != nil {
		_ = e.End(context.WithoutCancel(ctx), i, r.ID, "failed")
		return nil, er
	}
	a, er := e.Propose(ctx, i, Call{RunID: r.ID, Tool: tool, Params: p, Key: ID()}, request)
	if er != nil {
		_ = e.End(context.WithoutCancel(ctx), i, r.ID, "failed")
		return nil, er
	}
	if a.Status == "succeeded" {
		if er = e.End(ctx, i, r.ID, "succeeded"); er != nil {
			return nil, er
		}
	}
	return map[string]any{"mode": "deterministic-mock", "run_id": r.ID, "action": a, "answer": a.Result, "awaiting_human": a.Status == "pending"}, nil
}
