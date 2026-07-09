package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	completionStdout      io.Writer = os.Stdout
	completionStderr      io.Writer = os.Stderr
	completionUserHomeDir           = os.UserHomeDir
	completionUserShell             = func() string { return os.Getenv("SHELL") }
	completionMkdirAll              = os.MkdirAll
	completionReadFile              = os.ReadFile
	completionWriteFile             = os.WriteFile
	completionStat                  = os.Stat
)

func completionCmd(root *cobra.Command) *cobra.Command {
	var install bool
	var shell string

	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Manage shell completion scripts",
		Long: `arrctl completion - Shell completion support

Options:
  --install       Install completion for the current shell profile
  --shell SHELL   Shell type: bash|zsh|fish|powershell`,
		Example: `  arrctl completion --install
  arrctl completion --install --shell bash
  arrctl completion --shell zsh`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return cobra.MaximumNArgs(1)(cmd, args)
			}
			if len(args) == 1 {
				_, err := normalizeCompletionShell(args[0])
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				shell = args[0]
			}
			resolvedShell, err := detectCompletionShell(shell)
			if err != nil {
				return err
			}
			if install {
				return installCompletion(root, resolvedShell)
			}
			return generateCompletion(root, resolvedShell)
		},
	}

	cmd.Flags().BoolVar(&install, "install", false, "Install completion for the current shell profile")
	cmd.Flags().StringVar(&shell, "shell", "", "Shell type: bash|zsh|fish|powershell")
	return cmd
}

func normalizeCompletionShell(shell string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "bash", "zsh", "fish", "powershell":
		return strings.ToLower(strings.TrimSpace(shell)), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
}

func detectCompletionShell(explicit string) (string, error) {
	if explicit != "" {
		return normalizeCompletionShell(explicit)
	}
	base := filepath.Base(strings.TrimSpace(completionUserShell()))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "", fmt.Errorf("could not detect shell. Use --shell bash|zsh|fish|powershell")
	}
	return normalizeCompletionShell(base)
}

func generateCompletion(root *cobra.Command, shell string) error {
	switch shell {
	case "bash":
		return root.GenBashCompletion(completionStdout)
	case "zsh":
		return root.GenZshCompletion(completionStdout)
	case "fish":
		return root.GenFishCompletion(completionStdout, true)
	case "powershell":
		return root.GenPowerShellCompletion(completionStdout)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
}

func installCompletion(root *cobra.Command, shell string) error {
	home, err := completionUserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	switch shell {
	case "bash":
		return installBashCompletion(root, home)
	case "zsh":
		return installZshCompletion(root, home)
	default:
		return fmt.Errorf("--install is currently supported for bash and zsh only")
	}
}

func installBashCompletion(root *cobra.Command, home string) error {
	profile := filepath.Join(home, ".bashrc")
	if _, err := completionStat(profile); err != nil && os.IsNotExist(err) {
		profile = filepath.Join(home, ".bash_profile")
	}
	completionDir := filepath.Join(home, ".config", "arrctl", "completions")
	if err := completionMkdirAll(completionDir, 0o755); err != nil {
		return fmt.Errorf("create completion dir: %w", err)
	}
	completionFile := filepath.Join(completionDir, "arrctl.bash")
	if err := writeGeneratedCompletion(root, "bash", completionFile); err != nil {
		return err
	}
	block := strings.Join([]string{
		"# >>> arrctl completion >>>",
		fmt.Sprintf("if [ -f %q ]; then", completionFile),
		fmt.Sprintf("    . %q", completionFile),
		"fi",
		"# <<< arrctl completion <<<",
	}, "\n")
	if err := upsertProfileBlock(profile, block); err != nil {
		return err
	}
	fmt.Fprintf(completionStderr, "Installed bash completion in %s\n", profile)
	fmt.Fprintf(completionStderr, "Run: source %s\n", profile)
	return nil
}

func installZshCompletion(root *cobra.Command, home string) error {
	profile := filepath.Join(home, ".zshrc")
	completionDir := filepath.Join(home, ".zsh", "completions")
	if err := completionMkdirAll(completionDir, 0o755); err != nil {
		return fmt.Errorf("create completion dir: %w", err)
	}
	completionFile := filepath.Join(completionDir, "_arrctl")
	if err := writeGeneratedCompletion(root, "zsh", completionFile); err != nil {
		return err
	}
	block := strings.Join([]string{
		"# >>> arrctl completion >>>",
		fmt.Sprintf("fpath=(%q $fpath)", completionDir),
		"autoload -Uz compinit",
		"compinit",
		"# <<< arrctl completion <<<",
	}, "\n")
	if err := upsertProfileBlock(profile, block); err != nil {
		return err
	}
	fmt.Fprintf(completionStderr, "Installed zsh completion in %s\n", profile)
	fmt.Fprintf(completionStderr, "Run: source %s\n", profile)
	return nil
}

func writeGeneratedCompletion(root *cobra.Command, shell, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create completion file: %w", err)
	}
	defer f.Close()

	prevOut := completionStdout
	completionStdout = f
	defer func() { completionStdout = prevOut }()

	if err := generateCompletion(root, shell); err != nil {
		return err
	}
	return f.Close()
}

func upsertProfileBlock(path, block string) error {
	content := ""
	if b, err := completionReadFile(path); err == nil {
		content = string(b)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read profile: %w", err)
	}

	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	skip := false
	for _, line := range lines {
		switch line {
		case "# >>> arrctl completion >>>":
			skip = true
			continue
		case "# <<< arrctl completion <<<":
			skip = false
			continue
		}
		if !skip {
			filtered = append(filtered, line)
		}
	}

	trimmed := strings.TrimRight(strings.Join(filtered, "\n"), "\n")
	if trimmed != "" {
		trimmed += "\n\n"
	}
	trimmed += block + "\n"

	if err := completionWriteFile(path, []byte(trimmed), 0o644); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	return nil
}
