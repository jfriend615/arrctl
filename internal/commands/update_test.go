package commands

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeVersion(t *testing.T) {
	if got := normalizeVersion("0.3.0"); got != "v0.3.0" {
		t.Fatalf("unexpected normalized version: %q", got)
	}
	if got := normalizeVersion("v0.3.0"); got != "v0.3.0" {
		t.Fatalf("unexpected normalized version: %q", got)
	}
}

func TestReleasePlatform(t *testing.T) {
	goos, goarch, err := releasePlatform("darwin", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if goos != "darwin" || goarch != "amd64" {
		t.Fatalf("unexpected platform: %s/%s", goos, goarch)
	}
}

func TestNewUpdaterConfiguresHTTPTimeout(t *testing.T) {
	oldExecutable := updateExecutable
	oldRuntimeGOOS := updateRuntimeGOOS
	oldRuntimeGOARCH := updateRuntimeGOARCH
	t.Cleanup(func() {
		updateExecutable = oldExecutable
		updateRuntimeGOOS = oldRuntimeGOOS
		updateRuntimeGOARCH = oldRuntimeGOARCH
	})

	exe := filepath.Join(t.TempDir(), "arrctl")
	if err := os.WriteFile(exe, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	updateExecutable = func() (string, error) { return exe, nil }
	updateRuntimeGOOS = "darwin"
	updateRuntimeGOARCH = "amd64"

	u, err := newUpdater()
	if err != nil {
		t.Fatal(err)
	}
	if u.httpClient.Timeout != 30*time.Second {
		t.Fatalf("expected 30s timeout, got %s", u.httpClient.Timeout)
	}
}

func TestResolveVersionFromRedirect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/jfriend615/arrctl/releases/tag/v1.2.3", http.StatusFound)
	}))
	defer ts.Close()

	u := &updater{
		latestURL:  ts.URL + "/releases/latest",
		httpClient: ts.Client(),
	}

	tag, noV, err := u.resolveVersion(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.2.3" || noV != "1.2.3" {
		t.Fatalf("unexpected version resolution: %s %s", tag, noV)
	}
}

func TestUpdaterUpdateInstallsReleaseBinary(t *testing.T) {
	tmpDir := t.TempDir()
	execPath := filepath.Join(tmpDir, "arrctl")
	if err := os.WriteFile(execPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	const versionTag = "v1.2.3"
	const versionNoV = "1.2.3"
	const archiveName = "arrctl_1.2.3_darwin_amd64.tar.gz"
	archiveBytes := buildReleaseArchive(t, versionNoV, "darwin", "amd64", "new-binary")
	sum := sha256.Sum256(archiveBytes)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			http.Redirect(w, r, "/jfriend615/arrctl/releases/tag/"+versionTag, http.StatusFound)
		case "/releases/download/" + versionTag + "/" + archiveName:
			_, _ = w.Write(archiveBytes)
		case "/releases/download/" + versionTag + "/SHA256SUMS":
			fmt.Fprintf(w, "%x  %s\n", sum, archiveName)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	u := &updater{
		latestURL:    ts.URL + "/releases/latest",
		downloadBase: ts.URL + "/releases/download",
		httpClient:   ts.Client(),
		goos:         "darwin",
		goarch:       "amd64",
		executable:   execPath,
		mkdirTemp:    os.MkdirTemp,
	}

	if err := u.Update(context.Background(), ""); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("unexpected executable contents: %q", string(got))
	}
}

func TestChecksumForArchive(t *testing.T) {
	tmpDir := t.TempDir()
	checksumPath := filepath.Join(tmpDir, "SHA256SUMS")
	if err := os.WriteFile(checksumPath, []byte("abcd1234  arrctl_1.2.3_darwin_amd64.tar.gz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := checksumForArchive(checksumPath, "arrctl_1.2.3_darwin_amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != "abcd1234" {
		t.Fatalf("unexpected checksum: %q", got)
	}
}

func TestResolveExecutableTargetResolvesSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target-arrctl")
	link := filepath.Join(tmpDir, "arrctl")
	if err := os.WriteFile(target, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	got, err := resolveExecutableTarget(link)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("expected symlink to resolve to %q, got %q", want, got)
	}
}

func TestReplaceExecutableFallsBackToSudoOnPermissionError(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "arrctl-new")
	dest := filepath.Join(tmpDir, "arrctl")
	if err := os.WriteFile(src, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	oldLookPath := updateLookPath
	oldCommand := updateCommand
	oldRename := updateRename
	t.Cleanup(func() {
		updateLookPath = oldLookPath
		updateCommand = oldCommand
		updateRename = oldRename
	})

	updateLookPath = func(name string) (string, error) {
		switch name {
		case "sudo", "install":
			return "/usr/bin/" + name, nil
		default:
			return "", exec.ErrNotFound
		}
	}

	var gotName string
	var gotArgs []string
	updateCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string{}, args...)
		return helperCommand(t, "0")
	}

	updateRename = func(oldpath, newpath string) error {
		return os.ErrPermission
	}

	if err := replaceExecutable(context.Background(), src, dest); err != nil {
		t.Fatal(err)
	}
	if gotName != "sudo" {
		t.Fatalf("expected sudo fallback, got %q", gotName)
	}
	wantArgs := []string{"install", "-m", "755", src, dest}
	if strings.Join(gotArgs, "|") != strings.Join(wantArgs, "|") {
		t.Fatalf("unexpected sudo args: %#v", gotArgs)
	}
	matches, err := filepath.Glob(filepath.Join(tmpDir, ".arrctl-update-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected staged temp files to be cleaned up, found %v", matches)
	}
}

func TestReplaceExecutableReturnsHelpfulErrorWithoutSudo(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "arrctl-new")
	dest := filepath.Join(tmpDir, "arrctl")
	if err := os.WriteFile(src, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	oldLookPath := updateLookPath
	oldStage := updateStageExecutable
	t.Cleanup(func() {
		updateLookPath = oldLookPath
		updateStageExecutable = oldStage
	})

	updateLookPath = func(name string) (string, error) { return "", exec.ErrNotFound }
	updateStageExecutable = func(src, destDir string) (string, error) {
		return "", os.ErrPermission
	}

	err := replaceExecutable(context.Background(), src, dest)
	if err == nil || !strings.Contains(err.Error(), "sudo is unavailable") {
		t.Fatalf("expected sudo-unavailable error, got %v", err)
	}
}

func TestStageExecutableCleansUpTempFileOnCopyError(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "arrctl-new")
	if err := os.WriteFile(src, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	oldCopy := updateCopy
	t.Cleanup(func() { updateCopy = oldCopy })
	updateCopy = func(io.Writer, io.Reader) (int64, error) {
		return 0, errors.New("copy failed")
	}

	_, err := stageExecutable(src, tmpDir)
	if err == nil || !strings.Contains(err.Error(), "copy failed") {
		t.Fatalf("expected copy failure, got %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(tmpDir, ".arrctl-update-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no leftover temp files, found %v", matches)
	}
}

func helperCommand(t *testing.T, exitCode string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", exitCode)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func buildReleaseArchive(t *testing.T, versionNoV, goos, goarch, contents string) []byte {
	t.Helper()

	var buf strings.Builder
	tmpArchive := filepath.Join(t.TempDir(), "arrctl.tar.gz")
	f, err := os.Create(tmpArchive)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	name := fmt.Sprintf("arrctl_%s_%s_%s/arrctl", versionNoV, goos, goarch)
	data := []byte(contents)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(tmpArchive)
	if err != nil {
		t.Fatal(err)
	}
	buf.Write(b)
	return []byte(buf.String())
}
