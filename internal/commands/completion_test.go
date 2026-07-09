package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionCmdAutoDetectsShell(t *testing.T) {
	oldStdout := completionStdout
	oldStderr := completionStderr
	oldShell := completionUserShell
	t.Cleanup(func() {
		completionStdout = oldStdout
		completionStderr = oldStderr
		completionUserShell = oldShell
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	completionStdout = &stdout
	completionStderr = &stderr
	completionUserShell = func() string { return "/bin/zsh" }

	cmd := completionCmd(rootCmd())
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "#compdef arrctl") {
		t.Fatalf("expected zsh completion output, got %q", stdout.String())
	}
}

func TestCompletionInstallBashWritesScriptAndProfileBlock(t *testing.T) {
	tmpHome := t.TempDir()

	oldHome := completionUserHomeDir
	oldStdout := completionStdout
	oldStderr := completionStderr
	t.Cleanup(func() {
		completionUserHomeDir = oldHome
		completionStdout = oldStdout
		completionStderr = oldStderr
	})

	completionUserHomeDir = func() (string, error) { return tmpHome, nil }
	completionStdout = &bytes.Buffer{}

	var stderr bytes.Buffer
	completionStderr = &stderr

	cmd := completionCmd(rootCmd())
	cmd.SetArgs([]string{"--install", "--shell", "bash"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	completionPath := filepath.Join(tmpHome, ".config", "arrctl", "completions", "arrctl.bash")
	data, err := os.ReadFile(completionPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "_init_completion") {
		t.Fatalf("expected bash completion script, got %q", string(data))
	}

	profilePath := filepath.Join(tmpHome, ".bash_profile")
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(profile), completionPath) {
		t.Fatalf("expected profile to source completion file, got %q", string(profile))
	}
	if !strings.Contains(stderr.String(), "Installed bash completion") {
		t.Fatalf("expected install message, got %q", stderr.String())
	}
}

func TestCompletionInstallZshWritesScriptAndProfileBlock(t *testing.T) {
	tmpHome := t.TempDir()

	oldHome := completionUserHomeDir
	oldStdout := completionStdout
	oldStderr := completionStderr
	t.Cleanup(func() {
		completionUserHomeDir = oldHome
		completionStdout = oldStdout
		completionStderr = oldStderr
	})

	completionUserHomeDir = func() (string, error) { return tmpHome, nil }
	completionStdout = &bytes.Buffer{}

	var stderr bytes.Buffer
	completionStderr = &stderr

	cmd := completionCmd(rootCmd())
	cmd.SetArgs([]string{"--install", "--shell", "zsh"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	completionPath := filepath.Join(tmpHome, ".zsh", "completions", "_arrctl")
	data, err := os.ReadFile(completionPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "#compdef arrctl") {
		t.Fatalf("expected zsh completion script, got %q", string(data))
	}

	profilePath := filepath.Join(tmpHome, ".zshrc")
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(profile), filepath.Join(tmpHome, ".zsh", "completions")) {
		t.Fatalf("expected profile to add completion dir to fpath, got %q", string(profile))
	}
	if !strings.Contains(stderr.String(), "Installed zsh completion") {
		t.Fatalf("expected install message, got %q", stderr.String())
	}
}

func TestCompletionInstallRejectsUnsupportedShell(t *testing.T) {
	cmd := completionCmd(rootCmd())
	cmd.SetArgs([]string{"--install", "--shell", "fish"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "supported for bash and zsh only") {
		t.Fatalf("expected unsupported install error, got %v", err)
	}
}

