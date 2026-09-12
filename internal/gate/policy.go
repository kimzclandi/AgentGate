package gate

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"time"
)

type Tool struct {
	Name        string         `json:"name"`
	Action      string         `json:"action"`
	Kind        string         `json:"resource_kind"`
	Risk        string         `json:"risk"`
	TimeoutMS   int            `json:"timeout_ms"`
	SideEffect  bool           `json:"side_effect"`
	InputSchema map[string]any `json:"input_schema"`
}

func Registry() []Tool {
	return []Tool{
		{"document.read", "document:read", "document", "low", 1000, false, schema(false)},
		{"ticket.read", "ticket:read", "ticket", "low", 1000, false, schema(false)},
		{"ticket.update", "ticket:write", "ticket", "high", 1000, true, schema(true)},
	}
}
func schema(write bool) map[string]any {
	p := map[string]any{"resource_id": map[string]any{"type": "string", "maxLength": 100}}
	r := []string{"resource_id"}
	if write {
		p["body"] = map[string]any{"type": "string", "maxLength": 4096}
		r = append(r, "body")
	}
	return map[string]any{"type": "object", "properties": p, "required": r, "additionalProperties": false}
}
func toolNamed(n string) (Tool, error) {
	for _, t := range Registry() {
		if t.Name == n {
			return t, nil
		}
	}
	return Tool{}, ErrDenied
}
func roleAllows(role, action string) bool {
	return role == "operator" || role == "reader" && (action == "document:read" || action == "ticket:read")
}

type Run struct {
	AutoFinish bool     `json:"-"`
	Audience   string   `json:"audience"`
	ID         string   `json:"id"`
	Tenant     string   `json:"tenant"`
	User       string   `json:"user"`
	Agent      string   `json:"agent"`
	Scopes     []string `json:"scopes"`
	Status     string   `json:"status"`
	Expires    int64    `json:"expires"`
	Steps      int      `json:"steps"`
	Workdir    string   `json:"-"`
}

func getRun(ctx context.Context, tx *sql.Tx, id string) (Run, error) {
	var r Run
	var s string
	e := tx.QueryRowContext(ctx, `SELECT id,tenant,user_id,agent,scopes,status,expires,steps,workdir,audience,auto_finish FROM runs WHERE id=?`, id).Scan(&r.ID, &r.Tenant, &r.User, &r.Agent, &s, &r.Status, &r.Expires, &r.Steps, &r.Workdir, &r.Audience, &r.AutoFinish)
	if e != nil {
		return r, ErrDenied
	}
	if json.Unmarshal([]byte(s), &r.Scopes) != nil {
		return r, ErrDenied
	}
	return r, nil
}
func authorize(ctx context.Context, tx *sql.Tx, i Identity, r Run, t Tool, res string, confirming bool) (string, int, error) {
	if r.Audience != "agentgate-tools" || r.User != i.User || r.Tenant != i.Tenant || r.Status != "running" || r.Expires <= time.Now().Unix() || !stepBudgetAllows(r.Steps, confirming) || !slices.Contains(r.Scopes, t.Action) {
		return "", 0, ErrDenied
	}
	var tenant, role, actions string
	var enabled, ae, pv int
	if tx.QueryRowContext(ctx, `SELECT tenant,role,enabled FROM principals WHERE id=?`, i.User).Scan(&tenant, &role, &enabled) != nil || enabled != 1 || tenant != i.Tenant || !roleAllows(role, t.Action) {
		return "", 0, ErrDenied
	}
	if tx.QueryRowContext(ctx, `SELECT actions,enabled FROM agents WHERE id=?`, r.Agent).Scan(&actions, &ae) != nil || ae != 1 {
		return "", 0, ErrDenied
	}
	var scopes []string
	if json.Unmarshal([]byte(actions), &scopes) != nil || !slices.Contains(scopes, t.Action) {
		return "", 0, ErrDenied
	}
	connector, err := connectorFor(t.Kind)
	if err != nil {
		return "", 0, ErrDenied
	}
	resource, err := connector.Load(ctx, tx, i.Tenant, res)
	if err != nil || resource.Kind != t.Kind || resource.Owner != i.User {
		return "", 0, ErrDenied
	}
	body, owner, kind := resource.Body, resource.Owner, resource.Kind
	if len(body) > 16384 {
		return "", 0, ErrDenied
	}
	if tx.QueryRowContext(ctx, `SELECT version FROM policy WHERE id=1`).Scan(&pv) != nil {
		return "", 0, ErrDenied
	}
	if !evaluate(PolicyInput{i, r, t, role, owner, kind, scopes, enabled == 1, ae == 1, time.Now().Unix()}, confirming) {
		return "", 0, ErrDenied
	}
	return body, pv, nil
}

// PolicyInput is the complete in-memory decision input; loading trusted attributes
// remains the responsibility of authorize within the transaction.
type PolicyInput struct {
	Identity                  Identity
	Run                       Run
	Tool                      Tool
	Role, Owner, Kind         string
	AgentScopes               []string
	UserEnabled, AgentEnabled bool
	Now                       int64
}

func stepBudgetAllows(steps int, confirming bool) bool {
	return steps >= 0 && (steps < 8 || confirming && steps == 8)
}

func Evaluate(p PolicyInput) bool { return evaluate(p, false) }

// Confirmation consumes the step reserved by its persisted proposal, not a new step.
func evaluate(p PolicyInput, confirming bool) bool {
	return p.UserEnabled && p.AgentEnabled && p.Run.User == p.Identity.User && p.Run.Tenant == p.Identity.Tenant && p.Run.Status == "running" && p.Run.Expires > p.Now && stepBudgetAllows(p.Run.Steps, confirming) && p.Owner == p.Identity.User && p.Kind == p.Tool.Kind && roleAllows(p.Role, p.Tool.Action) && slices.Contains(p.Run.Scopes, p.Tool.Action) && slices.Contains(p.AgentScopes, p.Tool.Action)
}
