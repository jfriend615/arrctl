package commands

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jfriend615/arrctl/internal/api"
)

func TestResolveQualityFallsBackFromMissingDefault(t *testing.T) {
	c := api.New("http://example.com", "k")
	c.HTTP = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(`[{"id":7,"name":"HD-1080p"},{"id":8,"name":"4K"}]`), nil
		}),
	}
	var stderr bytes.Buffer
	id, err := resolveQuality(context.Background(), c, "", "Missing", &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if id != 7 {
		t.Fatalf("expected first available profile, got %d", id)
	}
	if !strings.Contains(stderr.String(), "configured default quality profile") {
		t.Fatalf("expected warning, got %q", stderr.String())
	}
}

func TestResolveRootFallsBackFromMissingDefault(t *testing.T) {
	c := api.New("http://example.com", "k")
	c.HTTP = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(`[{"path":"/tv"},{"path":"/media"}]`), nil
		}),
	}
	var stderr bytes.Buffer
	path, err := resolveRoot(context.Background(), c, "", "/missing", &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tv" {
		t.Fatalf("expected first available root, got %q", path)
	}
	if !strings.Contains(stderr.String(), "configured default root folder") {
		t.Fatalf("expected warning, got %q", stderr.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
