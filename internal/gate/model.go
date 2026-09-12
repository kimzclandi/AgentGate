package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// ModelPlanner has no store, identity credentials, or approval capability.
// It proposes exactly one call. The engine independently authorizes that call.
type ModelPlanner struct {
	endpoint, key, model string
	client               *http.Client
}

func publicIP(ip net.IP) bool {
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsUnspecified() {
		return false
	}
	for _, s := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/32", "2002::/16", "64:ff9b::/96"} {
		if netip.MustParsePrefix(s).Contains(a) {
			return false
		}
	}
	return true
}
func NewModel(endpoint, key, model, allowedHost string) (*ModelPlanner, error) {
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.Hostname() != allowedHost || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" && u.Port() != "443" || key == "" || model == "" {
		return nil, errors.New("invalid_model_config")
	}
	transport := &http.Transport{Proxy: nil, MaxConnsPerHost: 4, MaxIdleConns: 4, TLSHandshakeTimeout: 3 * time.Second, ResponseHeaderTimeout: 4 * time.Second, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(addr)
		if e != nil || host != allowedHost || port != "443" {
			return nil, errors.New("egress_denied")
		}
		ips, e := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if e != nil {
			return nil, e
		}
		if len(ips) == 0 {
			return nil, errors.New("egress_denied")
		}
		for _, ip := range ips {
			if !publicIP(ip) {
				return nil, errors.New("egress_denied")
			}
		}
		d := net.Dialer{Timeout: 3 * time.Second}
		return d.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}}
	return &ModelPlanner{endpoint, key, model, &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect_denied") }}}, nil
}
func (m *ModelPlanner) Plan(ctx context.Context, task string) (Call, error) {
	if task == "" || len(task) > 4096 {
		return Call{}, errors.New("invalid_task")
	}
	var ts []any
	for _, t := range Registry() {
		ts = append(ts, map[string]any{"type": "function", "function": map[string]any{"name": strings.ReplaceAll(t.Name, ".", "_"), "description": t.Action, "parameters": t.InputSchema}})
	}
	b, _ := json.Marshal(map[string]any{"model": m.model, "messages": []any{map[string]string{"role": "system", "content": "Propose one registered tool call for the user task. Tool calls are proposals only; never claim approval or execution."}, map[string]string{"role": "user", "content": task}}, "tools": ts, "tool_choice": "required", "parallel_tool_calls": false})
	req, e := http.NewRequestWithContext(ctx, "POST", m.endpoint, bytes.NewReader(b))
	if e != nil {
		return Call{}, e
	}
	req.Header.Set("Authorization", "Bearer "+m.key)
	req.Header.Set("Content-Type", "application/json")
	// No automatic retries: timeout may already have incurred provider work/cost.
	resp, e := m.client.Do(req)
	if e != nil {
		return Call{}, errors.New("model_result_unknown")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Call{}, errors.New("model_dependency_failure")
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, 16385))
	if e != nil || len(data) > 16384 {
		return Call{}, errors.New("model_output_limit")
	}
	var response struct {
		Choices []struct {
			Message struct {
				Calls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(data, &response) != nil || len(response.Choices) != 1 || len(response.Choices[0].Message.Calls) != 1 {
		return Call{}, errors.New("invalid_model_output")
	}
	f := response.Choices[0].Message.Calls[0].Function
	c := Call{Tool: strings.ReplaceAll(f.Name, "_", "."), Key: ID()}
	d := json.NewDecoder(strings.NewReader(f.Arguments))
	d.DisallowUnknownFields()
	if d.Decode(&c.Params) != nil || d.Decode(&struct{}{}) != io.EOF {
		return Call{}, errors.New("invalid_model_output")
	}
	if _, _, e = canonical(c); e != nil {
		return Call{}, e
	}
	return c, nil
}
func (e *Engine) ModelTask(ctx context.Context, i Identity, m *ModelPlanner, task, request string) (any, error) {
	c, er := m.Plan(ctx, task)
	if er != nil {
		return nil, er
	}
	t, er := toolNamed(c.Tool)
	if er != nil {
		return nil, er
	}
	r, er := e.CreateRun(ctx, i, "assistant", []string{t.Action}, 60)
	if er != nil {
		return nil, er
	}
	c.RunID = r.ID
	if _, er = e.Store.DB.ExecContext(ctx, `UPDATE runs SET auto_finish=1 WHERE id=?`, r.ID); er != nil {
		_ = e.End(context.WithoutCancel(ctx), i, r.ID, "failed")
		return nil, er
	}
	a, er := e.Propose(ctx, i, c, request)
	if er != nil {
		_ = e.End(context.WithoutCancel(ctx), i, r.ID, "failed")
		return nil, er
	}
	if a.Status == "succeeded" {
		if er = e.End(ctx, i, r.ID, "succeeded"); er != nil {
			return nil, er
		}
	}
	return map[string]any{"mode": "remote-model", "run_id": r.ID, "action": a, "answer": a.Result, "awaiting_human": a.Status == "pending"}, nil
}
