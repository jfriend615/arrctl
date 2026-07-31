package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestDecodeJSONPreservesDynamicNumberPrecision(t *testing.T) {
	var out map[string]any
	if err := decodeJSON([]byte(`{"percent":66.66666666666667}`), &out); err != nil {
		t.Fatal(err)
	}
	n, ok := out["percent"].(json.Number)
	if !ok || n.String() != "66.66666666666667" {
		t.Fatalf("expected preserved json.Number, got %#v", out["percent"])
	}
	var encoded bytes.Buffer
	if err := json.NewEncoder(&encoded).Encode(out); err != nil {
		t.Fatal(err)
	}
	if encoded.String() != "{\"percent\":66.66666666666667}\n" {
		t.Fatalf("unexpected re-encoded JSON: %s", encoded.String())
	}
}

func TestDecodeJSONRejectsTrailingData(t *testing.T) {
	for _, payload := range []string{
		`{"ok":true} {"second":true}`,
		`{"ok":true} trailing`,
	} {
		var out map[string]any
		if err := decodeJSON([]byte(payload), &out); err == nil {
			t.Fatalf("expected trailing data rejection for %q", payload)
		}
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

func TestTautulliBuildsQueryAndPreservesJSONNumbers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2" || r.URL.Query().Get("apikey") != "abc" || r.URL.Query().Get("cmd") != "get_library_media_info" || r.URL.Query().Get("section_id") != "7" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":{"result":"success","data":{"value":66.66666666666667}}}`))
	}))
	defer ts.Close()

	c := New(ts.URL, "abc")
	var out map[string]any
	if err := c.Tautulli(context.Background(), "get_library_media_info", map[string]string{"section_id": "7"}, &out); err != nil {
		t.Fatal(err)
	}
	response := out["response"].(map[string]any)
	data := response["data"].(map[string]any)
	if n, ok := data["value"].(json.Number); !ok || n.String() != "66.66666666666667" {
		t.Fatalf("unexpected decoded value: %#v", data["value"])
	}
}

func TestDo_PropagatesBodyReadError(t *testing.T) {
	c := New("http://example.com", "abc")
	c.HTTP = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       errReadCloser{err: errors.New("read failed")},
				Header:     make(http.Header),
			}, nil
		}),
	}
	err := c.Do(context.Background(), "GET", "/x", nil, nil)
	if err == nil || err.Error() != "read failed" {
		t.Fatalf("expected body read error, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errReadCloser struct {
	err error
}

func (e errReadCloser) Read([]byte) (int, error) {
	return 0, e.err
}

func (e errReadCloser) Close() error {
	return nil
}
