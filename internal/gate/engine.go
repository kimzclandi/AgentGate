package gate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var validID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

type Params struct {
	ResourceID string `json:"resource_id"`
	Body       string `json:"body,omitempty"`
}
type Call struct {
	RunID  string `json:"run_id"`
	Tool   string `json:"tool"`
	Params Params `json:"params"`
	Key    string `json:"idempotency_key"`
}
type Action struct {
	ID      string `json:"id"`
	RunID   string `json:"run_id"`
	Tool    string `json:"tool"`
	Params  Params `json:"params"`
	Digest  string `json:"digest"`
	Status  string `json:"status"`
	Expires int64  `json:"expires"`
	Result  string `json:"result,omitempty"`
}
type Engine struct {
	Store *Store
	Root  string
}

type requestKey struct{}

func audit(ctx context.Context, tx *sql.Tx, i Identity, r Run, res, action, outcome, aid, request string, pv int) error {
	if req, ok := ctx.Value(requestKey{}).(string); ok {
		request = req
	}
	if e := tx.QueryRowContext(ctx, `SELECT version FROM policy WHERE id=1`).Scan(&pv); e != nil {
		return e
	}
	_, e := tx.ExecContext(ctx, `INSERT INTO audit(at,request_id,tenant,user_id,agent,run_id,resource,action,policy_version,outcome,action_id) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, time.Now().Unix(), request, i.Tenant, i.User, r.Agent, r.ID, res, action, pv, outcome, aid)
	return e
}
func (e *Engine) CreateRun(ctx context.Context, i Identity, agent string, scopes []string, ttl int) (Run, error) {
	if ttl < 1 || ttl > 300 || len(scopes) == 0 || len(scopes) > 3 {
		return Run{}, errors.New("invalid_input")
	}
	tx, er := e.Store.DB.BeginTx(ctx, nil)
	if er != nil {
		return Run{}, er
	}
	defer tx.Rollback()
	var actions, role, tenant string
	var enabled int
	if tx.QueryRowContext(ctx, `SELECT actions FROM agents WHERE id=? AND enabled=1`, agent).Scan(&actions) != nil {
		return Run{}, ErrDenied
	}
	if tx.QueryRowContext(ctx, `SELECT role,tenant FROM principals WHERE id=? AND enabled=1`, i.User).Scan(&role, &tenant) != nil || tenant != i.Tenant {
		return Run{}, ErrDenied
	}
	var allowed []string
	if json.Unmarshal([]byte(actions), &allowed) != nil {
		return Run{}, ErrDenied
	}
	for _, s := range scopes {
		ok := false
		for _, a := range allowed {
			if a == s {
				ok = true
			}
		}
		if !ok || !roleAllows(role, s) {
			return Run{}, ErrDenied
		}
	}
	if er = tx.QueryRowContext(ctx, `SELECT count(*) FROM runs WHERE status='running' AND expires>?`, time.Now().Unix()).Scan(&enabled); er != nil {
		return Run{}, er
	}
	if enabled >= 16 {
		return Run{}, errors.New("capacity_exceeded")
	}
	id := ID()
	dir := filepath.Join(e.Root, id)
	if er = os.Mkdir(dir, 0700); er != nil {
		return Run{}, er
	}
	committed := false
	defer func() {
		if !committed {
			os.RemoveAll(dir)
		}
	}()
	r := Run{Audience: "agentgate-tools", ID: id, Tenant: i.Tenant, User: i.User, Agent: agent, Scopes: scopes, Status: "running", Expires: time.Now().Unix() + int64(ttl), Workdir: dir}
	b, _ := json.Marshal(scopes)
	_, er = tx.ExecContext(ctx, `INSERT INTO runs(id,tenant,user_id,agent,scopes,status,expires,workdir) VALUES(?,?,?,?,?,?,?,?)`, r.ID, r.Tenant, r.User, r.Agent, string(b), r.Status, r.Expires, r.Workdir)
	if er != nil {
		return Run{}, er
	}
	if er = audit(ctx, tx, i, r, "", "run.create", "allowed", "", ID(), 1); er != nil {
		return Run{}, er
	}
	er = tx.Commit()
	committed = er == nil
	return r, er
}
func canonical(c Call) (string, string, error) {
	t, er := toolNamed(c.Tool)
	if er != nil {
		return "", "", er
	}
	if !validID.MatchString(c.Params.ResourceID) || len(c.Params.Body) > 4096 || t.SideEffect && c.Params.Body == "" || !t.SideEffect && c.Params.Body != "" {
		return "", "", errors.New("invalid_input")
	}
	b, _ := json.Marshal(c.Params)
	return string(b), t.Action, nil
}
func binding(i Identity, c Call, p string) string {
	b, _ := json.Marshal([]string{i.Tenant, i.User, c.RunID, c.Tool, p})
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func (e *Engine) Propose(ctx context.Context, i Identity, c Call, request string) (Action, error) {
	p, _, er := canonical(c)
	if er != nil {
		return Action{}, er
	}
	t, _ := toolNamed(c.Tool)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(t.TimeoutMS)*time.Millisecond)
	defer cancel()
	tx, er := e.Store.DB.BeginTx(ctx, nil)
	if er != nil {
		return Action{}, er
	}
	defer tx.Rollback()
	r, er := getRun(ctx, tx, c.RunID)
	if er != nil {
		return Action{}, er
	}
	body, pv, er := authorize(ctx, tx, i, r, t, c.Params.ResourceID, false)
	if er != nil {
		if x := audit(ctx, tx, i, Run{ID: c.RunID}, "", t.Action, "denied", "", request, 0); x != nil {
			return Action{}, x
		}
		if x := tx.Commit(); x != nil {
			return Action{}, x
		}
		return Action{}, ErrDenied
	}
	a := Action{ID: ID(), RunID: r.ID, Tool: t.Name, Params: c.Params, Digest: binding(i, c, p), Status: "pending", Expires: time.Now().Add(2 * time.Minute).Unix()}
	if a.Expires > r.Expires {
		a.Expires = r.Expires
	}
	if t.SideEffect {
		if !validID.MatchString(c.Key) {
			return Action{}, errors.New("invalid_idempotency_key")
		}
		var existing, digest, status, result string
		var expires int64
		er = tx.QueryRowContext(ctx, `SELECT id,digest,status,expires,result FROM actions WHERE tenant=? AND user_id=? AND idem=?`, i.Tenant, i.User, c.Key).Scan(&existing, &digest, &status, &expires, &result)
		if er == nil {
			if digest != a.Digest {
				return Action{}, errors.New("idempotency_conflict")
			}
			a.ID = existing
			a.Status = status
			a.Expires = expires
			a.Result = result
			return a, nil
		}
		if er != sql.ErrNoRows {
			return Action{}, er
		}
		_, er = tx.ExecContext(ctx, `INSERT INTO actions(id,tenant,user_id,run_id,tool,params,digest,status,expires,idem) VALUES(?,?,?,?,?,?,?,?,?,?)`, a.ID, i.Tenant, i.User, r.ID, t.Name, p, a.Digest, a.Status, a.Expires, c.Key)
	} else {
		a.Status = "succeeded"
		a.Result = body
	}
	if er != nil {
		return Action{}, er
	}
	if _, er = tx.ExecContext(ctx, `UPDATE runs SET steps=steps+1 WHERE id=?`, r.ID); er != nil {
		return Action{}, er
	}
	if er = audit(ctx, tx, i, r, c.Params.ResourceID, t.Action, a.Status, a.ID, request, pv); er != nil {
		return Action{}, er
	}
	return a, tx.Commit()
}
func (e *Engine) Confirm(ctx context.Context, i Identity, id, digest, request string) (Action, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, er := e.Store.DB.BeginTx(ctx, nil)
	if er != nil {
		return Action{}, er
	}
	defer tx.Rollback()
	var a Action
	var p string
	er = tx.QueryRowContext(ctx, `SELECT id,run_id,tool,params,digest,status,expires,result FROM actions WHERE id=? AND tenant=? AND user_id=?`, id, i.Tenant, i.User).Scan(&a.ID, &a.RunID, &a.Tool, &p, &a.Digest, &a.Status, &a.Expires, &a.Result)
	if er != nil {
		return a, ErrDenied
	}
	if a.Status != "pending" {
		return Action{}, errors.New("approval_replayed")
	}
	if a.Expires <= time.Now().Unix() {
		return Action{}, errors.New("approval_expired")
	}
	if digest != a.Digest {
		return Action{}, errors.New("approval_mismatch")
	}
	if er = json.Unmarshal([]byte(p), &a.Params); er != nil {
		return Action{}, er
	}
	c := Call{RunID: a.RunID, Tool: a.Tool, Params: a.Params}
	if binding(i, c, p) != a.Digest {
		return Action{}, errors.New("approval_mismatch")
	}
	r, er := getRun(ctx, tx, a.RunID)
	if er != nil {
		return a, er
	}
	t, er := toolNamed(a.Tool)
	if er != nil || !t.SideEffect {
		return Action{}, ErrDenied
	}
	_, pv, er := authorize(ctx, tx, i, r, t, a.Params.ResourceID, true)
	if er != nil {
		return Action{}, er
	}
	connector, er := connectorFor(t.Kind)
	if er != nil {
		return Action{}, er
	}
	if er = connector.Apply(ctx, tx, i, a.Params); er != nil {
		return Action{}, er
	}
	a.Status = "succeeded"
	a.Result = "updated"
	_, er = tx.ExecContext(ctx, `UPDATE actions SET status='succeeded',result='updated' WHERE id=? AND status='pending'`, a.ID)
	if er != nil {
		return a, er
	}
	if er = audit(ctx, tx, i, r, a.Params.ResourceID, t.Action, "succeeded", a.ID, request, pv); er != nil {
		return a, er
	}
	if r.AutoFinish {
		if _, er = tx.ExecContext(ctx, `UPDATE runs SET status='succeeded' WHERE id=?`, r.ID); er != nil {
			return Action{}, er
		}
		if er = audit(ctx, tx, i, r, "", "run.end", "succeeded", a.ID, request, pv); er != nil {
			return Action{}, er
		}
	}
	if er = tx.Commit(); er != nil {
		return Action{}, er
	}
	if r.AutoFinish {
		_ = os.RemoveAll(filepath.Join(e.Root, r.ID))
	} // sweeper retries failed cleanup
	return a, nil
}
func (e *Engine) End(ctx context.Context, i Identity, id, status string) error {
	if status != "cancelled" && status != "succeeded" && status != "failed" {
		return errors.New("invalid_input")
	}
	tx, er := e.Store.DB.BeginTx(ctx, nil)
	if er != nil {
		return er
	}
	defer tx.Rollback()
	r, er := getRun(ctx, tx, id)
	if er != nil || r.User != i.User || r.Tenant != i.Tenant {
		return ErrDenied
	}
	if r.Status != "running" {
		return errors.New("run_terminal")
	}
	if _, er = tx.ExecContext(ctx, `UPDATE runs SET status=? WHERE id=?`, status, id); er != nil {
		return er
	}
	if er = audit(ctx, tx, i, r, "", "run.end", status, "", ID(), 1); er != nil {
		return er
	}
	if er = tx.Commit(); er != nil {
		return er
	}
	return os.RemoveAll(filepath.Join(e.Root, r.ID))
}
func (e *Engine) Revoke(ctx context.Context, i Identity) error {
	tx, er := e.Store.DB.BeginTx(ctx, nil)
	if er != nil {
		return er
	}
	defer tx.Rollback()
	if _, er = tx.ExecContext(ctx, `UPDATE principals SET enabled=0 WHERE id=? AND tenant=?; UPDATE policy SET version=version+1 WHERE id=1; UPDATE runs SET status='revoked' WHERE user_id=? AND tenant=? AND status='running'`, i.User, i.Tenant, i.User, i.Tenant); er != nil {
		return er
	}
	if er = audit(ctx, tx, i, Run{}, "", "identity.revoke", "revoked", "", ID(), 0); er != nil {
		return er
	}
	return tx.Commit()
}
func (e *Engine) Sweep(ctx context.Context) error {
	_, er := e.Store.DB.ExecContext(ctx, `UPDATE runs SET status='timed_out' WHERE status='running' AND expires<=?; UPDATE actions SET status='expired' WHERE status='pending' AND expires<=?`, time.Now().Unix(), time.Now().Unix())
	if er != nil {
		return er
	}
	return e.Store.Cleanup(e.Root)
}
