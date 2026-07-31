package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestCompletionPositionalHelpMatchesLegacyCLI(t *testing.T) {
	cmd := rootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"completion", "help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Shell completion support") {
		t.Fatalf("expected completion help, got %q", stdout.String())
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

	cmd = completionCmd(rootCmd())
	cmd.SetArgs([]string{"--install", "--shell", "bash"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(completionPath + ".bak.*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("idempotent reinstall created backups: %v", backups)
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

func TestCompletionInstallBacksUpUnmanagedZshFile(t *testing.T) {
	tmpHome := t.TempDir()
	completionPath := filepath.Join(tmpHome, ".zsh", "completions", "_arrctl")
	if err := os.MkdirAll(filepath.Dir(completionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(completionPath, []byte("# user-managed completion\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldHome := completionUserHomeDir
	oldStdout := completionStdout
	oldStderr := completionStderr
	oldNow := completionNow
	t.Cleanup(func() {
		completionUserHomeDir = oldHome
		completionStdout = oldStdout
		completionStderr = oldStderr
		completionNow = oldNow
	})
	completionUserHomeDir = func() (string, error) { return tmpHome, nil }
	completionStdout = &bytes.Buffer{}
	var stderr bytes.Buffer
	completionStderr = &stderr
	completionNow = func() time.Time { return time.Date(2026, 7, 30, 12, 34, 56, 0, time.UTC) }

	cmd := completionCmd(rootCmd())
	cmd.SetArgs([]string{"--install", "--shell", "zsh"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	backup := completionPath + ".bak.20260730123456"
	got, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# user-managed completion\n" {
		t.Fatalf("unexpected backup contents: %q", got)
	}
	if !strings.Contains(stderr.String(), "backed up existing completion file") {
		t.Fatalf("expected backup warning, got %q", stderr.String())
	}
}

func TestCompletionInstallReplacesLegacyZshSymlinkWithoutChangingTarget(t *testing.T) {
	tmpHome := t.TempDir()
	legacyTarget := filepath.Join(t.TempDir(), "_arrctl")
	legacyContents := "#compdef arrctl\n# tracked legacy completion\n"
	if err := os.WriteFile(legacyTarget, []byte(legacyContents), 0o644); err != nil {
		t.Fatal(err)
	}
	completionPath := filepath.Join(tmpHome, ".zsh", "completions", "_arrctl")
	if err := os.MkdirAll(filepath.Dir(completionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(legacyTarget, completionPath); err != nil {
		t.Fatal(err)
	}

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
	completionStderr = &bytes.Buffer{}

	cmd := completionCmd(rootCmd())
	cmd.SetArgs([]string{"--install", "--shell", "zsh"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(legacyTarget); err != nil || string(got) != legacyContents {
		t.Fatalf("legacy target changed: contents=%q err=%v", got, err)
	}
	info, err := os.Lstat(completionPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("expected legacy symlink to be replaced by a generated file")
	}
	generated, err := os.ReadFile(completionPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "#compdef arrctl") {
		t.Fatalf("unexpected generated completion: %q", generated)
	}
}
