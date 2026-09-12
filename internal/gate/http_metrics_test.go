package gate

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHTTPMetricsIncludeEntryRejections(t *testing.T) {
	e, a, _ := fixture(t)
	s := NewServer(e, a, nil)
	token, err := a.Token("alice")
	if err != nil {
		t.Fatal(err)
	}
	request := func(path string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		if authenticated {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if w := request("/api/me", false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request("/api/unknown", true); w.Code != 404 {
		t.Fatal(w.Code)
	}
	for n := 0; n < cap(s.slots); n++ {
		s.slots <- struct{}{}
	}
	if w := request("/api/me", true); w.Code != 429 {
		t.Fatal(w.Code)
	}
	for len(s.slots) > 0 {
		<-s.slots
	}
	if w := request("/api/overview?offset=invalid", true); w.Code != 400 {
		t.Fatal(w.Code)
	}
	w := request("/api/metrics", true)
	var result struct{ Requests, Errors int }
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Requests != 5 || result.Errors != 4 {
		t.Fatal(result)
	}
}
