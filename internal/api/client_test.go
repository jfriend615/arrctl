package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDo_SetsHeaderAndParsesJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "abc" {
			t.Fatalf("missing api key header")
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()
	c := New(ts.URL, "abc")
	var out map[string]any
	if err := c.Do(context.Background(), "GET", "/x", nil, &out); err != nil {
		t.Fatal(err)
	}
}

func TestDo_HandlesAuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer ts.Close()
	c := New(ts.URL, "abc")
	if err := c.Do(context.Background(), "GET", "/x", nil, nil); err == nil {
		t.Fatal("expected error")
	}
}
