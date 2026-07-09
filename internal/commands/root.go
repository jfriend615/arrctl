package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgPath string
	format  string
	quiet   bool
	version = "dev"
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func exitErr(code int, err error) error { return &exitError{code: code, err: err} }

func Execute() {
	if err := rootCmd().Execute(); err != nil {
		if ee, ok := err.(*exitError); ok {
			if ee.err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", ee.err)
			}
			os.Exit(ee.code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "arrctl",
		Short:         "Unified CLI for *arr",
		SilenceErrors: true,
		SilenceUsage:  true,
		Long: `arrctl - Unified CLI for managing *arr services

Commands:
  sonarr      Manage Sonarr (TV shows)
  radarr      Manage Radarr (Movies)
  overseerr   Manage Overseerr (Requests)
  tautulli    View Plex activity (who is streaming)
  calendar    Show upcoming releases (TV + Movies)
  completion  Manage shell completion scripts
  update      Update arrctl to the latest release

Configuration:
  Default config: $XDG_CONFIG_HOME/arrctl/config.json or ~/.config/arrctl/config.json
  Or set environment variables:
    ARRCTL_CONFIG
    SONARR_URL / SONARR_API_KEY
    RADARR_URL / RADARR_API_KEY
    OVERSEERR_URL / OVERSEERR_API_KEY
    TAUTULLI_URL / TAUTULLI_API_KEY`,
		Example: `  arrctl sonarr list
  arrctl sonarr search "Breaking Bad"
  arrctl radarr add --id 603 --search
  arrctl overseerr pending
  arrctl tautulli stale --library Movies --min-days 365 --max-plays 1
  arrctl calendar --days 14 --sonarr
  arrctl completion --install`,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpCommand(&cobra.Command{Hidden: true})
	root.PersistentFlags().StringVar(&cfgPath, "config", "", "Config file")
	root.PersistentFlags().StringVar(&format, "format", "auto", "json|table|auto")
	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode")
	root.Version = version
	root.AddCommand(sonarrCmd(), radarrCmd(), overseerrCmd(), tautulliCmd(), calendarCmd(), completionCmd(root), updateCmd())
	return root
}
