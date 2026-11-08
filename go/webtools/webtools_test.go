package webtools

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewEmptyURLIsNil(t *testing.T) {
	if New("  ", "k") != nil {
		t.Fatal("empty URL must disable web tools")
	}
}

func TestExecuteSearchAndFetch(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if strings.Contains(gotBody, "boom") {
			http.Error(w, `{"error":"bad"}`, http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	c := New(srv.URL+"/", "secret")

	if _, err := c.Execute(ToolSearch, `{"query":"weather"}`); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/search" || gotAuth != "Bearer secret" || !strings.Contains(gotBody, `"max_results":5`) {
		t.Fatalf("search request wrong: %s %s %s", gotPath, gotAuth, gotBody)
	}
	if _, err := c.Execute(ToolFetch, `{"url":"https://example.com"}`); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/fetch" || !strings.Contains(gotBody, `"max_chars":6000`) {
		t.Fatalf("fetch request wrong: %s %s", gotPath, gotBody)
	}
	if _, err := c.Execute(ToolSearch, `{"query":"boom"}`); err == nil {
		t.Fatal("non-200 must be an error")
	}
	if _, err := c.Execute("rm_rf", `{}`); err == nil {
		t.Fatal("unknown tool must be an error")
	}
	if _, err := c.Execute(ToolSearch, `not json`); err == nil {
		t.Fatal("bad arguments must be an error")
	}
}
