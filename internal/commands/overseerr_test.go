package commands

import (
	"bytes"
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
