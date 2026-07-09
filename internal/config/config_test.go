package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_PrefersFlagThenEnvThenDefault(t *testing.T) {
	d := t.TempDir()
	cfg := filepath.Join(d, "config.json")
	if err := os.WriteFile(cfg, []byte(`{"sonarr":{"url":"http://file","api_key":"k1"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARRCTL_CONFIG", filepath.Join(d, "missing.json"))
	c, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.MustService("sonarr")
	if err != nil {
		t.Fatal(err)
	}
	if s.URL != "http://file" || s.APIKey != "k1" {
		t.Fatalf("unexpected service: %+v", s)
	}
}

func TestLoad_EnvOverlay(t *testing.T) {
	d := t.TempDir()
	cfg := filepath.Join(d, "config.json")
	if err := os.WriteFile(cfg, []byte(`{"sonarr":{"url":"http://file","api_key":"k1"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SONARR_URL", "http://env")
	t.Setenv("SONARR_API_KEY", "k2")
	c, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := c.MustService("sonarr")
	if s.URL != "http://env" || s.APIKey != "k2" {
		t.Fatalf("overlay failed: %+v", s)
	}
}

func TestLoad_DefaultPathUsesXDGConfigHome(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", d)
	path := filepath.Join(d, "arrctl", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"sonarr":{"url":"http://xdg","api_key":"k"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.MustService("sonarr")
	if err != nil {
		t.Fatal(err)
	}
	if s.URL != "http://xdg" || s.APIKey != "k" {
		t.Fatalf("unexpected service: %+v", s)
	}
}
