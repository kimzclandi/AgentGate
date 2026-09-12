package gate

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNetworkPolicy(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "::ffff:127.0.0.1", "192.168.1.1", "0.0.0.0", "2001:db8::1"} {
		if publicIP(net.ParseIP(ip)) {
			t.Fatal("private/reserved allowed", ip)
		}
	}
	if !publicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address rejected")
	}
	for _, endpoint := range []string{"http://example.com/v1/chat/completions", "https://evil.com/v1/chat/completions", "https://user:pass@example.com/path", "https://example.com:444/path"} {
		if _, e := NewModel(endpoint, "key", "model", "example.com"); e == nil {
			t.Fatal("invalid endpoint", endpoint)
		}
	}
	m, e := NewModel("https://127.0.0.1/v1/chat/completions", "key", "model", "127.0.0.1")
	if e != nil {
		t.Fatal(e)
	}
	_, e = m.Plan(context.Background(), "test")
	if e == nil {
		t.Fatal("loopback egress allowed")
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestModelUntrustedAndNoRetry(t *testing.T) {
	var calls atomic.Int32
	m := &ModelPlanner{endpoint: "https://example.com/", key: "test", model: "test", client: &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) { calls.Add(1); return nil, context.DeadlineExceeded })}}
	if _, e := m.Plan(context.Background(), "read document"); e == nil || e.Error() != "model_result_unknown" {
		t.Fatal(e)
	}
	if calls.Load() != 1 {
		t.Fatal("unknown outcome retried")
	}
	for _, args := range []string{`{"resource_id":"doc-1","tenant_id":"beta"}`, `{"resource_id":"../secret"}`} {
		m.client.Transport = roundTripper(func(r *http.Request) (*http.Response, error) {
			body := `{"choices":[{"message":{"tool_calls":[{"function":{"name":"document_read","arguments":` + strconvQuote(args) + `}}]}}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		if _, e := m.Plan(context.Background(), "read"); e == nil {
			t.Fatal("unsafe model plan allowed")
		}
	}
}
func strconvQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}
func TestModelTimeoutCancellation(t *testing.T) {
	var calls atomic.Int32
	m := &ModelPlanner{endpoint: "https://example.com/", key: "test", model: "test", client: &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}}
	ctx, c := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer c()
	start := time.Now()
	if _, e := m.Plan(ctx, "test"); e == nil {
		t.Fatal("timeout ignored")
	}
	if time.Since(start) > time.Second || calls.Load() != 1 {
		t.Fatal("unbounded timeout/retries")
	}
}
