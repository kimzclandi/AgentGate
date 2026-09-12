package gate

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type Server struct {
	Chat     *ChatService
	Model    *ModelPlanner
	Engine   *Engine
	Auth     *Auth
	Web      http.Handler
	slots    chan struct{}
	requests atomic.Int64
	failures atomic.Int64
	latency  atomic.Int64
}

func NewServer(e *Engine, a *Auth, web http.Handler) *Server {
	return &Server{Engine: e, Auth: a, Web: web, slots: make(chan struct{}, 32)}
}
func out(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return errors.New("invalid_input")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("invalid_input")
	}
	return nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.requests.Add(1)
	defer func() { s.latency.Add(time.Since(start).Microseconds()) }()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("Cache-Control", "no-store")
	request := ID()
	w.Header().Set("X-Request-ID", request)
	if r.URL.Path == "/healthz" {
		out(w, 200, map[string]string{"status": "alive"})
		return
	}
	if r.URL.Path == "/readyz" {
		ctx, c := context.WithTimeout(r.Context(), time.Second)
		defer c()
		if s.Engine.Store.DB.PingContext(ctx) != nil {
			out(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		out(w, 200, map[string]string{"status": "ready"})
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		s.Web.ServeHTTP(w, r)
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		out(w, 429, map[string]string{"error": "capacity_exceeded"})
		return
	}
	timeout := 5 * time.Second
	if r.URL.Path == "/api/chat" || r.URL.Path == "/api/chat/resume" {
		timeout = 245 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	ctx = context.WithValue(ctx, requestKey{}, request)
	r = r.WithContext(ctx)
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if raw == r.Header.Get("Authorization") {
		out(w, 401, map[string]string{"error": "unauthenticated"})
		return
	}
	i, er := s.Auth.Verify(ctx, raw)
	if er != nil {
		out(w, 401, map[string]string{"error": "unauthenticated"})
		return
	}
	var result any
	switch r.Method + " " + r.URL.Path {
	case "GET /api/capabilities":
		result = map[string]any{"local_model": s.Chat != nil, "remote_model": s.Model != nil, "mock": true}
	case "GET /api/chat":
		if s.Chat == nil {
			er = errors.New("local_model_unavailable")
		} else {
			result, er = s.Chat.Get(ctx, i, r.URL.Query().Get("id"))
		}
	case "POST /api/chat":
		var b struct {
			Task string `json:"task"`
		}
		er = decode(w, r, &b)
		if er == nil {
			if s.Chat == nil {
				er = errors.New("local_model_unavailable")
			} else {
				result, er = s.Chat.Start(ctx, i, b.Task)
			}
		}
	case "POST /api/chat/resume":
		var b struct {
			ID string `json:"chat_id"`
		}
		er = decode(w, r, &b)
		if er == nil {
			if s.Chat == nil {
				er = errors.New("local_model_unavailable")
			} else {
				result, er = s.Chat.Resume(ctx, i, b.ID)
			}
		}
	case "POST /api/agent/remote":
		var b struct {
			Task string `json:"task"`
		}
		er = decode(w, r, &b)
		if er == nil {
			if s.Model == nil {
				er = errors.New("model_not_configured")
			} else {
				result, er = s.Engine.ModelTask(ctx, i, s.Model, b.Task, request)
			}
		}
	case "GET /api/me":
		result = i
	case "GET /api/tools":
		result = Registry()
	case "GET /api/metrics":
		result = map[string]any{"requests": s.requests.Load(), "errors": s.failures.Load(), "request_latency_total_us": s.latency.Load(), "inflight": len(s.slots), "queue_capacity": 0, "note": "process-local HTTP totals; tenant business counters are in overview"}
	case "POST /api/runs":
		var b struct {
			Agent  string   `json:"agent"`
			Scopes []string `json:"scopes"`
			TTL    int      `json:"ttl_seconds"`
		}
		er = decode(w, r, &b)
		if er == nil {
			result, er = s.Engine.CreateRun(ctx, i, b.Agent, b.Scopes, b.TTL)
		}
	case "POST /api/tools/call":
		var b Call
		er = decode(w, r, &b)
		if er == nil {
			result, er = s.Engine.Propose(ctx, i, b, request)
		}
	case "POST /api/approve":
		var b struct {
			ID     string `json:"action_id"`
			Digest string `json:"digest"`
		}
		er = decode(w, r, &b)
		if er == nil {
			result, er = s.Engine.Confirm(ctx, i, b.ID, b.Digest, request)
		}
	case "POST /api/cancel":
		var b struct {
			ID string `json:"run_id"`
		}
		er = decode(w, r, &b)
		if er == nil {
			er = s.Engine.End(ctx, i, b.ID, "cancelled")
			result = map[string]string{"status": "cancelled"}
		}
	case "POST /api/revoke":
		er = s.Engine.Revoke(ctx, i)
		result = map[string]string{"status": "revoked"}
	case "POST /api/agent":
		var b struct {
			Task string `json:"task"`
		}
		er = decode(w, r, &b)
		if er == nil {
			result, er = s.Engine.MockTask(ctx, i, b.Task, request)
		}
	case "GET /api/overview":
		result, er = s.overview(ctx, i, r)
	default:
		out(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if er != nil {
		s.failures.Add(1)
		status := 409
		code := er.Error()
		switch {
		case errors.Is(er, ErrDenied):
			status = 403
			code = "access_denied"
		case strings.HasPrefix(code, "invalid"):
			status = 400
		case code == "local_model_unavailable":
			status = 503
		case code == "chat_busy" || code == "chat_not_resumable" || code == "approval_required" || code == "step_limit" || code == "conversation_limit" || code == "write_must_be_single_call":
			status = 409
		case code == "model_result_unknown":
			status = 502
		case code == "capacity_exceeded":
			status = 429
		case errors.Is(er, context.DeadlineExceeded):
			status = 504
			code = "timeout"
		case code == "approval_replayed" || code == "approval_expired" || code == "approval_mismatch" || code == "idempotency_conflict" || code == "run_terminal":
		default:
			status = 503
			code = "dependency_unavailable"
		}

		// Record rejected requests without resource existence or parameter details.
		// If the DB is unavailable, deny the operation; no fake audit success.
		if tx, x := s.Engine.Store.DB.BeginTx(ctx, nil); x == nil {
			if x = audit(ctx, tx, i, Run{}, "", r.URL.Path, code, "", request, 0); x == nil {
				_ = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
		}
		out(w, status, map[string]string{"error": code, "request_id": request})
		return
	}
	out(w, 200, result)
}
func (s *Server) overview(ctx context.Context, i Identity, r *http.Request) (any, error) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 || offset > 100000 {
		return nil, errors.New("invalid_offset")
	}
	db := s.Engine.Store.DB
	result := map[string]any{"identity": i, "tools": Registry(), "offset": offset, "limit": 50}
	queries := map[string]string{
		"chats":     `SELECT id,run_id,status,answer,pending FROM chats WHERE tenant=? AND user_id=? ORDER BY rowid DESC LIMIT 50 OFFSET ?`,
		"runs":      `SELECT id,agent,status,expires,steps FROM runs WHERE tenant=? AND user_id=? ORDER BY rowid DESC LIMIT 50 OFFSET ?`,
		"approvals": `SELECT id,run_id,tool,params,digest,status,expires FROM actions WHERE tenant=? AND user_id=? ORDER BY rowid DESC LIMIT 50 OFFSET ?`,
		"audit":     `SELECT seq,at,request_id,agent,run_id,resource,action,policy_version,outcome,action_id FROM audit WHERE tenant=? AND user_id=? ORDER BY seq DESC LIMIT 50 OFFSET ?`,
	}
	for name, q := range queries {
		rows, e := db.QueryContext(ctx, q, i.Tenant, i.User, offset)
		if e != nil {
			return nil, e
		}
		cols, e := rows.Columns()
		if e != nil {
			rows.Close()
			return nil, e
		}
		items := []map[string]any{}
		for rows.Next() {
			vals := make([]any, len(cols))
			ptr := make([]any, len(cols))
			for j := range vals {
				ptr[j] = &vals[j]
			}
			if e = rows.Scan(ptr...); e != nil {
				rows.Close()
				return nil, e
			}
			item := map[string]any{}
			for j, c := range cols {
				item[c] = vals[j]
			}
			items = append(items, item)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		result[name] = items
	}
	var pv int
	if e := db.QueryRowContext(ctx, `SELECT version FROM policy WHERE id=1`).Scan(&pv); e != nil {
		return nil, e
	}
	result["policy"] = map[string]any{"version": pv, "default": "deny", "constraints": []string{"same tenant", "resource owner", "user role ∩ agent actions ∩ delegation", "active run; live identity; max 8 steps"}, "cache": "none"}
	return result, nil
}
