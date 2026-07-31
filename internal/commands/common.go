package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/jfriend615/arrctl/internal/api"
	"github.com/jfriend615/arrctl/internal/config"
	"github.com/jfriend615/arrctl/internal/output"
	"github.com/spf13/cobra"
)

var printJSON = output.PrintJSON

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
		return printJSON(v)
	}
	output.PrintTable(headers, rows)
	return nil
}

func parentCommandArgs(name string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || (len(args) == 1 && args[0] == "help") {
			return nil
		}
		return fmt.Errorf("unknown %s command: %s", name, args[0])
	}
}

func validateFormat(v string) error {
	switch output.Mode(v) {
	case output.Auto, output.Table, output.JSON:
		return nil
	default:
		return fmt.Errorf("invalid format: %s (use json|table|auto)", v)
	}
}

func resolveQuality(ctx context.Context, c *api.Client, explicit, def string, warn io.Writer) (int, error) {
	var p []profile
	if err := c.Do(ctx, "GET", "/api/v3/qualityprofile", nil, &p); err != nil {
		return 0, err
	}
	if explicit != "" {
		if n, err := strconv.Atoi(explicit); err == nil {
			for _, pr := range p {
				if pr.ID == n {
					return n, nil
				}
			}
		}
		for _, pr := range p {
			if pr.Name == explicit {
				return pr.ID, nil
			}
		}
		return 0, fmt.Errorf("quality profile not found: %s", explicit)
	}
	if def != "" {
		for _, pr := range p {
			if pr.Name == def {
				return pr.ID, nil
			}
		}
		if !quiet && warn != nil {
			fmt.Fprintf(warn, "Warning: configured default quality profile %q not found, using first available\n", def)
		}
	}
	if len(p) == 0 {
		return 0, errors.New("no quality profiles available")
	}
	return p[0].ID, nil
}

func resolveRoot(ctx context.Context, c *api.Client, explicit, def string, warn io.Writer) (string, error) {
	var r []rootFolder
	if err := c.Do(ctx, "GET", "/api/v3/rootfolder", nil, &r); err != nil {
		return "", err
	}
	if explicit != "" {
		for _, rr := range r {
			if rr.Path == explicit {
				return explicit, nil
			}
		}
		return "", fmt.Errorf("root folder not found: %s", explicit)
	}
	if def != "" {
		for _, rr := range r {
			if rr.Path == def {
				return def, nil
			}
		}
		if !quiet && warn != nil {
			fmt.Fprintf(warn, "Warning: configured default root folder %q not found, using first available\n", def)
		}
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
	case json.Number:
		n, _ := t.Int64()
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
