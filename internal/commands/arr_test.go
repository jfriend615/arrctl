package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jfriend615/arrctl/internal/api"
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

func TestListAutoNonTTYPreservesRawJSONFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v3/series" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1,"title":"Example","monitored":true,"futureApiField":"preserved"}]`))
	}))
	defer ts.Close()

	configPath := writeArrTestConfig(t, "sonarr", ts.URL)
	oldPrinter := printJSON
	var captured any
	printJSON = func(v any) error {
		captured = v
		return nil
	}
	t.Cleanup(func() { printJSON = oldPrinter })

	cmd := rootCmd()
	cmd.SetArgs([]string{"--config", configPath, "--format", "auto", "sonarr", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	items, ok := captured.([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected rendered value: %#v", captured)
	}
	if items[0]["futureApiField"] != "preserved" {
		t.Fatalf("expected unmodeled API field to survive auto JSON: %#v", items[0])
	}
}

func TestInfoJSONMatchesBashSchemas(t *testing.T) {
	tests := []struct {
		name      string
		service   string
		mediaPath string
		mediaJSON string
		extraPath string
		extraJSON string
		wantKeys  []string
	}{
		{
			name:      "sonarr",
			service:   "sonarr",
			mediaPath: "/api/v3/series",
			mediaJSON: `[{"id":42,"title":"Severance","year":2022,"status":"continuing","monitored":true,"qualityProfileId":7,"overview":"Work is mysterious","seasons":[{}]}]`,
			extraPath: "/api/v3/episode",
			extraJSON: `[{"id":1},{"id":2}]`,
			wantKeys:  []string{"episodesCount", "id", "monitored", "overview", "qualityProfileName", "seasonsCount", "status", "title", "year"},
		},
		{
			name:      "radarr",
			service:   "radarr",
			mediaPath: "/api/v3/movie",
			mediaJSON: `[{"id":100,"title":"Dune","year":2021,"status":"released","monitored":true,"qualityProfileId":7,"overview":"Spice","movieFileId":9}]`,
			extraPath: "/api/v3/moviefile/9",
			extraJSON: `{"id":9,"size":1610612736}`,
			wantKeys:  []string{"id", "monitored", "movieSizeGb", "overview", "qualityProfileName", "status", "title", "year"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case tt.mediaPath:
					_, _ = w.Write([]byte(tt.mediaJSON))
				case "/api/v3/qualityprofile":
					_, _ = w.Write([]byte(`[{"id":7,"name":"HD-1080p"}]`))
				case tt.extraPath:
					_, _ = w.Write([]byte(tt.extraJSON))
				default:
					t.Fatalf("unexpected extra request: %s", r.URL.String())
				}
			}))
			defer ts.Close()

			configPath := writeArrTestConfig(t, tt.service, ts.URL)
			oldPrinter := printJSON
			var captured any
			printJSON = func(v any) error {
				captured = v
				return nil
			}
			t.Cleanup(func() { printJSON = oldPrinter })

			cmd := rootCmd()
			cmd.SetArgs([]string{"--config", configPath, "--format", "json", tt.service, "info", "--id", map[string]string{"sonarr": "42", "radarr": "100"}[tt.service]})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			out, ok := captured.([]map[string]any)
			if !ok || len(out) != 1 {
				t.Fatalf("unexpected rendered value: %#v", captured)
			}
			gotKeys := make([]string, 0, len(out[0]))
			for key := range out[0] {
				gotKeys = append(gotKeys, key)
			}
			sort.Strings(gotKeys)
			if !reflect.DeepEqual(gotKeys, tt.wantKeys) {
				t.Fatalf("schema mismatch\n got: %v\nwant: %v", gotKeys, tt.wantKeys)
			}
		})
	}
}

func TestRadarrInfoFetchesPartialEmbeddedMovieFile(t *testing.T) {
	movieFileFetched := false
	c := api.New("http://radarr.test", "test-key")
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := ""
		switch r.URL.Path {
		case "/api/v3/qualityprofile":
			body = `[{"id":7,"name":"HD-1080p"}]`
		case "/api/v3/moviefile/9":
			movieFileFetched = true
			body = `{"id":9,"size":1610612736}`
		default:
			return nil, errors.New("unexpected request: " + r.URL.String())
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})

	oldPrinter := printJSON
	oldFormat := format
	var captured any
	printJSON = func(v any) error {
		captured = v
		return nil
	}
	format = "json"
	t.Cleanup(func() {
		printJSON = oldPrinter
		format = oldFormat
	})

	cmd := rootCmd()
	cmd.SetContext(context.Background())
	matched := []arrItem{{ID: 100, Title: "Dune", QualityProfileID: 7, MovieFile: &movieFile{ID: 9}}}
	if err := renderMovieInfo(cmd, c, matched); err != nil {
		t.Fatal(err)
	}
	if !movieFileFetched {
		t.Fatal("expected partial embedded movie file to be fetched")
	}
	items, ok := captured.([]map[string]any)
	if !ok || len(items) != 1 || items[0]["movieSizeGb"] != 1.5 {
		t.Fatalf("unexpected movie info: %#v", captured)
	}
}

func TestConfirmDeletionFailsFastWithoutTerminal(t *testing.T) {
	_, err := confirmDeletionWithTTY("sonarr", "series", "Example", 42, func() (io.ReadCloser, error) {
		return nil, errors.New("no tty")
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "use --yes") {
		t.Fatalf("expected non-terminal guidance, got %v", err)
	}
}

func TestLeafCommandsRejectUnexpectedArguments(t *testing.T) {
	tests := [][]string{
		{"sonarr", "list", "junk"},
		{"sonarr", "add", "junk", "--id", "1"},
		{"sonarr", "info", "junk", "--id", "1"},
		{"sonarr", "delete", "junk", "--id", "1", "--yes"},
		{"sonarr", "calendar", "junk"},
		{"tautulli", "now", "junk"},
		{"tautulli", "stale", "junk"},
		{"overseerr", "pending", "junk"},
		{"update", "junk"},
	}
	for _, args := range tests {
		cmd := rootCmd()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("args %v: expected rejection", args)
		}
	}
}

func writeArrTestConfig(t *testing.T, service, url string) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.json")
	configJSON := `{"` + service + `":{"url":"` + url + `","api_key":"test-key"}}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath
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

func TestRadarrAddMatchesBashRequestContract(t *testing.T) {
	var posted map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/movie/lookup":
			_, _ = w.Write([]byte(`[{"title":"The Matrix","tmdbId":603,"images":[]}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/qualityprofile":
			_, _ = w.Write([]byte(`[{"id":7,"name":"HD-1080p"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/rootfolder":
			_, _ = w.Write([]byte(`[{"path":"/movies"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/movie":
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":44,"title":"The Matrix"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer ts.Close()

	configPath := writeArrTestConfig(t, "radarr", ts.URL)
	cmd := rootCmd()
	cmd.SetArgs([]string{"--config", configPath, "radarr", "add", "--id", "603", "--root", "/movies", "--quality", "7", "--search", "--no-monitored"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if posted["title"] != "The Matrix" || posted["tmdbId"] != float64(603) {
		t.Fatalf("unexpected identity fields: %#v", posted)
	}
	if posted["monitored"] != false || posted["rootFolderPath"] != "/movies" || posted["qualityProfileId"] != float64(7) {
		t.Fatalf("unexpected add settings: %#v", posted)
	}
	addOptions, ok := posted["addOptions"].(map[string]any)
	if !ok || addOptions["searchForMovie"] != true {
		t.Fatalf("unexpected add options: %#v", posted["addOptions"])
	}
	if _, exists := posted["seasonFolder"]; exists {
		t.Fatalf("Radarr payload included Sonarr-only fields: %#v", posted)
	}
}

func TestDeleteMatchesBashHTTPAndJSONContract(t *testing.T) {
	tests := []struct {
		service string
		noun    string
	}{
		{service: "sonarr", noun: "series"},
		{service: "radarr", noun: "movie"},
	}
	for _, tt := range tests {
		t.Run(tt.service, func(t *testing.T) {
			deleteSeen := false
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/v3/"+tt.noun+"/42":
					_, _ = w.Write([]byte(`{"id":42,"title":"Disposable"}`))
				case r.Method == http.MethodDelete && r.URL.Path == "/api/v3/"+tt.noun+"/42":
					deleteSeen = true
					if r.URL.Query().Get("deleteFiles") != "true" || r.URL.Query().Get("addImportListExclusion") != "true" {
						t.Fatalf("unexpected delete query: %s", r.URL.RawQuery)
					}
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				}
			}))
			defer ts.Close()

			oldPrinter := printJSON
			var captured any
			printJSON = func(v any) error {
				captured = v
				return nil
			}
			t.Cleanup(func() { printJSON = oldPrinter })

			cmd := rootCmd()
			cmd.SetArgs([]string{"--config", writeArrTestConfig(t, tt.service, ts.URL), "--format", "json", tt.service, "delete", "--id", "42", "--delete-files", "--add-exclusion", "--yes"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !deleteSeen {
				t.Fatal("delete request was not made")
			}
			out, ok := captured.(map[string]any)
			if !ok || out["service"] != tt.service || out["id"] != 42 || out["title"] != "Disposable" || out["deleteFiles"] != true || out["addImportListExclusion"] != true {
				t.Fatalf("unexpected delete JSON: %#v", captured)
			}
		})
	}
}
