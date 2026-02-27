package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Service struct {
	URL      string `json:"url"`
	APIKey   string `json:"api_key"`
	Defaults struct {
		QualityProfile string `json:"qualityProfile"`
		RootFolder     string `json:"rootFolder"`
	} `json:"defaults"`
}

type Config struct {
	Sonarr    Service `json:"sonarr"`
	Radarr    Service `json:"radarr"`
	Overseerr Service `json:"overseerr"`
	Tautulli  Service `json:"tautulli"`
}

func defaultConfigPath() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "arrctl", "config.json")
}

func Load(pathFlag string) (Config, error) {
	var cfg Config
	path := strings.TrimSpace(pathFlag)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("ARRCTL_CONFIG"))
	}
	if path == "" {
		path = defaultConfigPath()
	}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return cfg, fmt.Errorf("invalid JSON in config %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}
	overlay(&cfg.Sonarr, "SONARR")
	overlay(&cfg.Radarr, "RADARR")
	overlay(&cfg.Overseerr, "OVERSEERR")
	overlay(&cfg.Tautulli, "TAUTULLI")
	return cfg, nil
}

func overlay(s *Service, prefix string) {
	if v := os.Getenv(prefix + "_URL"); v != "" {
		s.URL = v
	}
	if v := os.Getenv(prefix + "_API_KEY"); v != "" {
		s.APIKey = v
	}
}

func (c Config) MustService(name string) (Service, error) {
	var s Service
	switch name {
	case "sonarr":
		s = c.Sonarr
	case "radarr":
		s = c.Radarr
	case "overseerr":
		s = c.Overseerr
	case "tautulli":
		s = c.Tautulli
	default:
		return Service{}, fmt.Errorf("unknown service: %s", name)
	}
	if s.URL == "" || s.APIKey == "" {
		return Service{}, fmt.Errorf("missing %s configuration", name)
	}
	return s, nil
}
