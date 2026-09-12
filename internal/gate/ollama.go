package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type ChatToolCall struct {
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}
type ChatMessage struct {
	Role     string         `json:"role"`
	Content  string         `json:"content"`
	ToolName string         `json:"tool_name,omitempty"`
	Calls    []ChatToolCall `json:"tool_calls,omitempty"`
}
type ChatModel interface {
	Next(context.Context, []ChatMessage) (ChatMessage, error)
}
type Ollama struct {
	model  string
	client *http.Client
}

// Local inference is an explicit adapter, not an exception in the remote SSRF
// policy. Its address is fixed; neither prompts nor request JSON can change it.
func NewOllama(model string) (*Ollama, error) {
	if model == "" || strings.ContainsAny(model, " \r\n/") || strings.Contains(model, "cloud") {
		return nil, errors.New("invalid_local_model")
	}
	return &Ollama{model: model, client: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect_denied") }, Transport: &http.Transport{Proxy: nil, MaxConnsPerHost: 2, MaxIdleConns: 2, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr != "127.0.0.1:11434" {
			return nil, errors.New("egress_denied")
		}
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "tcp", addr)
	}}}}, nil
}
func (o *Ollama) Next(ctx context.Context, messages []ChatMessage) (ChatMessage, error) {
	var ts []any
	for _, t := range Registry() {
		ts = append(ts, map[string]any{"type": "function", "function": map[string]any{"name": strings.ReplaceAll(t.Name, ".", "_"), "description": localToolDescription(t.Name), "parameters": t.InputSchema}})
	}
	b, e := json.Marshal(map[string]any{"model": o.model, "messages": messages, "tools": ts, "stream": false, "think": false, "keep_alive": "5m", "options": map[string]any{"temperature": 0, "num_ctx": 8192, "num_predict": 1024}})
	if e != nil || len(b) > 65536 {
		return ChatMessage{}, errors.New("conversation_limit")
	}
	req, e := http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1:11434/api/chat", bytes.NewReader(b))
	if e != nil {
		return ChatMessage{}, e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := o.client.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return ChatMessage{}, ctx.Err()
		}
		return ChatMessage{}, errors.New("local_model_unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ChatMessage{}, errors.New("local_model_unavailable")
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, 32769))
	if e != nil || len(data) > 32768 {
		return ChatMessage{}, errors.New("model_output_limit")
	}
	var v struct {
		Message ChatMessage `json:"message"`
		Done    bool        `json:"done"`
	}
	if json.Unmarshal(data, &v) != nil || !v.Done || v.Message.Role != "assistant" || len(v.Message.Content) > 16384 || len(v.Message.Calls) > 4 {
		return ChatMessage{}, errors.New("invalid_model_output")
	}
	return v.Message, nil
}
func parseChatCall(call ChatToolCall) (Call, error) {
	c := Call{Tool: strings.ReplaceAll(call.Function.Name, "_", "."), Key: ID()}
	d := json.NewDecoder(bytes.NewReader(call.Function.Arguments))
	d.DisallowUnknownFields()
	if d.Decode(&c.Params) != nil || d.Decode(&struct{}{}) != io.EOF {
		return c, errors.New("invalid_model_output")
	}
	_, _, e := canonical(c)
	return c, e
}

func localToolDescription(name string) string {
	switch name {
	case "document.read":
		return "Read a document by its exact resource_id. Only accepts document resources, not tickets. Returns the actual document text."
	case "ticket.read":
		return "Read/query a ticket by its exact resource_id. Use this for ticket contents and status, not document_read. Returns the current ticket text."
	case "ticket.update":
		return "Replace a ticket body with the supplied text. This proposes a write requiring human approval; do not claim completion until the tool returns success."
	default:
		return name
	}
}
