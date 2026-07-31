package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOverseerrPendingPrintsNoPendingRequests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/request" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	configPath := writeOverseerrTestConfig(t, ts.URL)

	oldCfgPath, oldFormat, oldQuiet := cfgPath, format, quiet
	t.Cleanup(func() {
		cfgPath = oldCfgPath
		format = oldFormat
		quiet = oldQuiet
	})
	cfgPath = ""
	format = "table"
	quiet = false

	cmd := rootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config", configPath, "overseerr", "pending"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "No pending requests") {
		t.Fatalf("expected no-pending message, got %q", stderr.String())
	}
}

func TestOverseerrDecisionBodiesMatchBash(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		path     string
		wantBody map[string]string
	}{
		{name: "approve", args: []string{"approve", "123", "--message", "Adding tonight"}, path: "/api/v1/request/123/approve", wantBody: map[string]string{"message": "Adding tonight"}},
		{name: "deny", args: []string{"deny", "125", "--reason", "Already available"}, path: "/api/v1/request/125/decline", wantBody: map[string]string{"reason": "Already available"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != tt.path {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				}
				defer r.Body.Close()
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatal(err)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()

			cmd := rootCmd()
			args := []string{"--config", writeOverseerrTestConfig(t, ts.URL), "overseerr"}
			args = append(args, tt.args...)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if len(gotBody) != 1 {
				t.Fatalf("unexpected body: %#v", gotBody)
			}
			for key, want := range tt.wantBody {
				if gotBody[key] != want {
					t.Fatalf("body mismatch: %#v", gotBody)
				}
			}
		})
	}
}

func TestOverseerrDenyPrintsDeclinedRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/request/125/decline" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	configPath := writeOverseerrTestConfig(t, ts.URL)

	oldCfgPath, oldFormat, oldQuiet := cfgPath, format, quiet
	t.Cleanup(func() {
		cfgPath = oldCfgPath
		format = oldFormat
		quiet = oldQuiet
	})
	cfgPath = ""
	format = "table"
	quiet = false

	cmd := rootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config", configPath, "overseerr", "deny", "125"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Declined request 125") {
		t.Fatalf("expected declined message, got %q", stderr.String())
	}
}

func TestOverseerrPendingExplicitJSONDoesNotResolveTitles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request" {
			t.Fatalf("explicit JSON made unexpected title request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":12,"type":"movie","media":{"tmdbId":603}}]}`))
	}))
	defer ts.Close()

	configPath := writeOverseerrTestConfig(t, ts.URL)
	oldPrinter := printJSON
	var captured any
	printJSON = func(v any) error {
		captured = v
		return nil
	}
	t.Cleanup(func() { printJSON = oldPrinter })

	cmd := rootCmd()
	cmd.SetArgs([]string{"--config", configPath, "--format", "json", "overseerr", "pending"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	results, ok := captured.([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("unexpected JSON result: %#v", captured)
	}
}

func TestOverseerrPendingAutoJSONAddsDisplayFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/request":
			_, _ = w.Write([]byte(`{"results":[{"id":12,"type":"movie","createdAt":"2026-07-30T12:00:00Z","requestedBy":{"displayName":"Jordan"},"media":{"tmdbId":603}}]}`))
		case "/api/v1/movie/603":
			_, _ = w.Write([]byte(`{"title":"The Matrix"}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
	}))
	defer ts.Close()

	configPath := writeOverseerrTestConfig(t, ts.URL)
	oldPrinter := printJSON
	var captured any
	printJSON = func(v any) error {
		captured = v
		return nil
	}
	t.Cleanup(func() { printJSON = oldPrinter })

	cmd := rootCmd()
	cmd.SetArgs([]string{"--config", configPath, "--format", "auto", "overseerr", "pending"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	results, ok := captured.([]map[string]any)
	if !ok || len(results) != 1 {
		t.Fatalf("unexpected auto JSON result: %#v", captured)
	}
	if results[0]["requestUser"] != "Jordan" || results[0]["requestTitle"] != "The Matrix" {
		t.Fatalf("missing display enrichment: %#v", results[0])
	}
}

func TestOverseerrPendingTreatsNullResultsAsEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":null}`))
	}))
	defer ts.Close()

	configPath := writeOverseerrTestConfig(t, ts.URL)
	cmd := rootCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config", configPath, "overseerr", "pending"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "No pending requests") {
		t.Fatalf("expected empty-results message, got %q", stderr.String())
	}
}

func writeOverseerrTestConfig(t *testing.T, url string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configJSON := `{
  "overseerr": {
    "url": "` + url + `",
    "api_key": "test-key"
  }
}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath
}
