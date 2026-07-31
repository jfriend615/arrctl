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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const updateRepo = "jfriend615/arrctl"

var (
	updateLatestURL     = "https://github.com/" + updateRepo + "/releases/latest"
	updateDownloadBase  = "https://github.com/" + updateRepo + "/releases/download"
	updateExecutable    = os.Executable
	updateMkdirTemp     = os.MkdirTemp
	updateHTTPClient    = func() *http.Client { return &http.Client{Timeout: 30 * time.Second} }
	updateRuntimeGOOS   = runtime.GOOS
	updateRuntimeGOARCH = runtime.GOARCH
	updateLookPath      = exec.LookPath
	updateCommand       = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, name, args...)
	}
	updateStageExecutable = stageExecutable
	updateRename          = os.Rename
	updateCopy            = io.Copy
)

func updateCmd() *cobra.Command {
	var targetVersion string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update arrctl to the latest release",
		Args:  cobra.NoArgs,
		Long: `arrctl update - Update arrctl to the latest release binary

This command downloads the latest published release for the current platform
and replaces the installed executable.`,
		Example: `  arrctl update
  arrctl update --version v0.3.0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := newUpdater()
			if err != nil {
				return exitErr(2, err)
			}
			if err := u.Update(cmd.Context(), targetVersion); err != nil {
				return exitErr(2, err)
			}
			if !quiet {
				msg := "Updated arrctl to the latest release"
				if strings.TrimSpace(targetVersion) != "" {
					msg = fmt.Sprintf("Updated arrctl to %s", normalizeVersion(targetVersion))
				}
				fmt.Fprintln(cmd.ErrOrStderr(), msg)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&targetVersion, "version", "", "Install a specific release version")
	return cmd
}

type updater struct {
	latestURL    string
	downloadBase string
	httpClient   *http.Client
	goos         string
	goarch       string
	executable   string
	mkdirTemp    func(string, string) (string, error)
}

func newUpdater() (*updater, error) {
	exe, err := updateExecutable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	resolvedExe, err := resolveExecutableTarget(exe)
	if err != nil {
		return nil, err
	}
	goos, goarch, err := releasePlatform(updateRuntimeGOOS, updateRuntimeGOARCH)
	if err != nil {
		return nil, err
	}
	return &updater{
		latestURL:    updateLatestURL,
		downloadBase: updateDownloadBase,
		httpClient:   updateHTTPClient(),
		goos:         goos,
		goarch:       goarch,
		executable:   resolvedExe,
		mkdirTemp:    updateMkdirTemp,
	}, nil
}

func (u *updater) Update(ctx context.Context, requestedVersion string) error {
	versionTag, versionNoV, err := u.resolveVersion(ctx, requestedVersion)
	if err != nil {
		return err
	}
	archiveName := fmt.Sprintf("arrctl_%s_%s_%s.tar.gz", versionNoV, u.goos, u.goarch)
	baseURL := strings.TrimRight(u.downloadBase, "/") + "/" + versionTag

	tmpDir, err := u.mkdirTemp("", "arrctl-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveName)
	if err := u.downloadFile(ctx, baseURL+"/"+archiveName, archivePath); err != nil {
		return err
	}

	checksumPath := filepath.Join(tmpDir, "SHA256SUMS")
	if err := u.downloadFile(ctx, baseURL+"/SHA256SUMS", checksumPath); err != nil {
		return err
	}
	if err := verifyChecksum(archivePath, checksumPath, archiveName); err != nil {
		return err
	}

	binaryPath, err := extractBinary(archivePath, tmpDir, versionNoV, u.goos, u.goarch)
	if err != nil {
		return err
	}
	if err := replaceExecutable(ctx, binaryPath, u.executable); err != nil {
		return err
	}
	return nil
}

func (u *updater) resolveVersion(ctx context.Context, requestedVersion string) (string, string, error) {
	if strings.TrimSpace(requestedVersion) != "" {
		tag := normalizeVersion(requestedVersion)
		return tag, strings.TrimPrefix(tag, "v"), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.latestURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("build latest release request: %w", err)
	}
	client := *u.httpClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("resolve latest release version: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		loc := resp.Header.Get("Location")
		if loc == "" {
			return "", "", errors.New("unable to resolve latest release version")
		}
		tag := pathBase(loc)
		if tag == "" {
			return "", "", errors.New("unable to resolve latest release version")
		}
		return tag, strings.TrimPrefix(tag, "v"), nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		tag := pathBase(resp.Request.URL.String())
		if tag == "" || tag == "latest" {
			return "", "", errors.New("unable to resolve latest release version")
		}
		return tag, strings.TrimPrefix(tag, "v"), nil
	default:
		return "", "", fmt.Errorf("resolve latest release version: unexpected HTTP %d", resp.StatusCode)
	}
}

func (u *updater) downloadFile(ctx context.Context, fileURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", fileURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download %s: unexpected HTTP %d", fileURL, resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return nil
}

func releasePlatform(goos, goarch string) (string, string, error) {
	switch goos {
	case "darwin", "linux":
	default:
		return "", "", fmt.Errorf("unsupported OS: %s (supported: darwin, linux)", goos)
	}

	switch goarch {
	case "x86_64":
		goarch = "amd64"
	case "aarch64":
		goarch = "arm64"
	case "amd64", "arm64":
	default:
		return "", "", fmt.Errorf("unsupported architecture: %s (supported: amd64, arm64)", goarch)
	}
	return goos, goarch, nil
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

func pathBase(raw string) string {
	u, err := url.Parse(raw)
	if err == nil {
		raw = u.Path
	}
	raw = strings.TrimRight(raw, "/")
	if raw == "" {
		return ""
	}
	idx := strings.LastIndex(raw, "/")
	if idx == -1 {
		return raw
	}
	return raw[idx+1:]
}

func resolveExecutableTarget(exe string) (string, error) {
	resolved, err := filepath.EvalSymlinks(exe)
	if err == nil {
		return resolved, nil
	}
	return "", fmt.Errorf("resolve executable target %s: %w", exe, err)
}

func verifyChecksum(archivePath, checksumPath, archiveName string) error {
	expected, err := checksumForArchive(checksumPath, archiveName)
	if err != nil {
		return err
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive for checksum verification: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	actual := fmt.Sprintf("%x", h.Sum(nil))
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("checksum mismatch for %s", archiveName)
	}
	return nil
}

func checksumForArchive(checksumPath, archiveName string) (string, error) {
	b, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", fmt.Errorf("read checksum file: %w", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[len(fields)-1] == archiveName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("could not find checksum for %s", archiveName)
}

func extractBinary(archivePath, destDir, versionNoV, goos, goarch string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("open gzip archive: %w", err)
	}
	defer gzr.Close()

	expectedName := fmt.Sprintf("arrctl_%s_%s_%s/arrctl", versionNoV, goos, goarch)
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || hdr.Name != expectedName {
			continue
		}
		out := filepath.Join(destDir, "arrctl")
		dst, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", fmt.Errorf("create extracted binary: %w", err)
		}
		if _, err := io.Copy(dst, tr); err != nil {
			dst.Close()
			return "", fmt.Errorf("extract binary: %w", err)
		}
		if err := dst.Close(); err != nil {
			return "", fmt.Errorf("close extracted binary: %w", err)
		}
		return out, nil
	}
	return "", errors.New("extracted archive missing arrctl binary")
}

func replaceExecutable(ctx context.Context, src, dest string) error {
	destDir := filepath.Dir(dest)
	tmpName, err := updateStageExecutable(src, destDir)
	if err == nil {
		defer os.Remove(tmpName)
		if err := updateRename(tmpName, dest); err == nil {
			return nil
		} else if !isPermissionError(err) {
			return fmt.Errorf("replace executable %s: %w", dest, err)
		}
	} else if !isPermissionError(err) {
		return err
	}

	if err := installWithSudo(ctx, src, dest); err != nil {
		return err
	}
	return nil
}

func stageExecutable(src, destDir string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open extracted binary: %w", err)
	}
	defer in.Close()

	tmp, err := os.CreateTemp(destDir, ".arrctl-update-*")
	if err != nil {
		return "", fmt.Errorf("create temporary executable: %w", err)
	}
	tmpName := tmp.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := updateCopy(tmp, in); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write temporary executable: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return "", fmt.Errorf("chmod temporary executable: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary executable: %w", err)
	}
	success = true
	return tmpName, nil
}

func installWithSudo(ctx context.Context, src, dest string) error {
	if _, err := updateLookPath("sudo"); err != nil {
		return fmt.Errorf("replace executable %s: permission denied and sudo is unavailable", dest)
	}
	if _, err := updateLookPath("install"); err != nil {
		return fmt.Errorf("replace executable %s: permission denied and install is unavailable", dest)
	}
	cmd := updateCommand(ctx, "sudo", "install", "-m", "755", src, dest)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("replace executable %s with sudo: %w", dest, err)
	}
	return nil
}

func isPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM)
}
