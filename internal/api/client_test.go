package api

import (
	"context"
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

func TestDo_HandlesAuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer ts.Close()
	c := New(ts.URL, "abc")
	if err := c.Do(context.Background(), "GET", "/x", nil, nil); err == nil {
		t.Fatal("expected error")
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
