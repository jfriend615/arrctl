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

func TestSearchRejectsNegativeLimit(t *testing.T) {
	oldCfgPath, oldFormat, oldQuiet := cfgPath, format, quiet
	t.Cleanup(func() {
		cfgPath = oldCfgPath
		format = oldFormat
		quiet = oldQuiet
	})

	cfgPath = ""
	format = "json"
	quiet = false

	cmd := rootCmd()
	cmd.SetArgs([]string{"sonarr", "search", "test", "--limit", "-1"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--limit must be >= 0") {
		t.Fatalf("expected negative limit error, got %v", err)
	}
}

func TestListRejectsConflictingMonitoredFlags(t *testing.T) {
	oldCfgPath, oldFormat, oldQuiet := cfgPath, format, quiet
	t.Cleanup(func() {
		cfgPath = oldCfgPath
		format = oldFormat
		quiet = oldQuiet
	})

	cfgPath = ""
	format = "json"
	quiet = false

	cmd := rootCmd()
	cmd.SetArgs([]string{"sonarr", "list", "--monitored", "--unmonitored"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--monitored and --unmonitored cannot be used together") {
		t.Fatalf("expected conflicting monitored flag error, got %v", err)
	}
}

func TestSonarrAddPrintsBashStyleProgress(t *testing.T) {
	t.Helper()

	var posted map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/series/lookup":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"title":"Severance","tvdbId":371980,"images":[],"seasons":[{"seasonNumber":1}]}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/qualityprofile":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":7,"name":"HD-1080p"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/rootfolder":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"path":"/tv"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/series":
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":323,"title":"Severance"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configJSON := `{
  "sonarr": {
    "url": "` + ts.URL + `",
    "api_key": "test-key",
    "defaults": {
      "qualityProfile": "HD-1080p",
      "rootFolder": "/tv"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	oldCfgPath, oldFormat, oldQuiet := cfgPath, format, quiet
	t.Cleanup(func() {
		cfgPath = oldCfgPath
		format = oldFormat
		quiet = oldQuiet
	})

	cfgPath = ""
	format = "json"
	quiet = false

	cmd := rootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config", configPath, "sonarr", "add", "--id", "371980", "--search"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got := stderr.String()
	for _, want := range []string{
		"Looking up series with TVDB ID: 371980",
		"Found: Severance",
		"Using quality profile ID: 7",
		"Using root folder: /tv",
		"Successfully added: Severance (ID: 323)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, got)
		}
	}

	if posted["title"] != "Severance" {
		t.Fatalf("unexpected title payload: %#v", posted["title"])
	}
	if posted["qualityProfileId"] != float64(7) {
		t.Fatalf("unexpected quality profile payload: %#v", posted["qualityProfileId"])
	}
	if posted["rootFolderPath"] != "/tv" {
		t.Fatalf("unexpected root folder payload: %#v", posted["rootFolderPath"])
	}
	addOptions, ok := posted["addOptions"].(map[string]any)
	if !ok || addOptions["searchForMissingEpisodes"] != true {
		t.Fatalf("unexpected addOptions payload: %#v", posted["addOptions"])
	}
}
