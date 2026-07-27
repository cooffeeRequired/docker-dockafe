package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Apply downloads assetURL, verifies expectedSHA256 (required), and replaces the running executable.
// The process should exit after a successful Apply so the new binary is used next launch.
func Apply(ctx context.Context, assetURL, expectedSHA256 string, client *http.Client) (string, error) {
	if assetURL == "" {
		return "", fmt.Errorf("missing download url")
	}
	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	if expectedSHA256 == "" {
		return "", fmt.Errorf("missing SHA256 checksum — refuse to install (need dockafe.sha256 on the release)")
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	if client == nil {
		client = &http.Client{Timeout: 3 * time.Minute}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "dockafe-updater")
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: %s", resp.Status)
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".dockafe-update-*")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, 200<<20)); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	f, err := os.Open(tmpName)
	if err != nil {
		return "", err
	}
	got, err := HashReader(f)
	_ = f.Close()
	if err != nil {
		return "", fmt.Errorf("hash download: %w", err)
	}
	if got != expectedSHA256 {
		return "", fmt.Errorf("SHA256 mismatch: got %s want %s", got, expectedSHA256)
	}

	if err := os.Chmod(tmpName, 0o755); err != nil {
		return "", err
	}

	backup := exe + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(exe, backup); err != nil {
		return "", fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(tmpName, exe); err != nil {
		_ = os.Rename(backup, exe) // best-effort rollback
		return "", fmt.Errorf("install new binary: %w", err)
	}
	cleanup = false
	_ = os.Remove(backup)
	return exe, nil
}
