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
	version = "0.2.1"
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }

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
	root := &cobra.Command{Use: "arrctl", Short: "Unified CLI for *arr", SilenceUsage: true}
	root.PersistentFlags().StringVar(&cfgPath, "config", "", "Config file")
	root.PersistentFlags().StringVar(&format, "format", "auto", "json|table|auto")
	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode")
	root.Version = version
	root.AddCommand(sonarrCmd(), radarrCmd(), overseerrCmd(), tautulliCmd(), calendarCmd(), completionCmd(root))
	return root
}

func completionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{Use: "completion [bash|zsh|fish|powershell]", Short: "Generate shell completion", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return root.GenBashCompletion(os.Stdout)
		case "zsh":
			return root.GenZshCompletion(os.Stdout)
		case "fish":
			return root.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return root.GenPowerShellCompletion(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell")
		}
	}}
	return cmd
}
