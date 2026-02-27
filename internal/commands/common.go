package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jfriend615/arrctl/internal/api"
	"github.com/jfriend615/arrctl/internal/config"
	"github.com/jfriend615/arrctl/internal/output"
)

func serviceClient(name string) (*api.Client, config.Service, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, config.Service{}, err
	}
	s, err := cfg.MustService(name)
	if err != nil {
		return nil, config.Service{}, err
	}
	return api.New(s.URL, s.APIKey), s, nil
}

func render(v any, headers []string, rows [][]string) error {
	m := output.Mode(format)
	if m == output.JSON || !output.IsTable(m) {
		return output.PrintJSON(v)
	}
	output.PrintTable(headers, rows)
	return nil
}

func resolveQuality(ctx context.Context, c *api.Client, explicit, def string) (int, error) {
	var p []profile
	if err := c.Do(ctx, "GET", "/api/v3/qualityprofile", nil, &p); err != nil {
		return 0, err
	}
	pick := explicit
	if pick == "" {
		pick = def
	}
	if pick != "" {
		if n, err := strconv.Atoi(pick); err == nil {
			for _, pr := range p {
				if pr.ID == n {
					return n, nil
				}
			}
		}
		for _, pr := range p {
			if pr.Name == pick {
				return pr.ID, nil
			}
		}
		return 0, fmt.Errorf("quality profile not found: %s", pick)
	}
	if len(p) == 0 {
		return 0, errors.New("no quality profiles available")
	}
	return p[0].ID, nil
}

func resolveRoot(ctx context.Context, c *api.Client, explicit, def string) (string, error) {
	var r []rootFolder
	if err := c.Do(ctx, "GET", "/api/v3/rootfolder", nil, &r); err != nil {
		return "", err
	}
	pick := explicit
	if pick == "" {
		pick = def
	}
	if pick != "" {
		for _, rr := range r {
			if rr.Path == pick {
				return pick, nil
			}
		}
		return "", fmt.Errorf("root folder not found: %s", pick)
	}
	if len(r) == 0 {
		return "", errors.New("no root folders configured")
	}
	return r[0].Path, nil
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}

func toMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
