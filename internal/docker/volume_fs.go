package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
)

const defaultVolumeHelperImage = "busybox:1.36.1"

// VolumeEntry is one file or directory inside a named volume.
type VolumeEntry struct {
	Name  string
	Path  string // relative to volume root, no leading slash
	IsDir bool
	Size  int64
}

func volumeHelperImage() string {
	if v := strings.TrimSpace(os.Getenv("DOCKAFE_BUSYBOX_IMAGE")); v != "" {
		return v
	}
	return defaultVolumeHelperImage
}

// IsRemoteDaemon reports whether the Docker API points at a non-local unix socket.
func (c *Client) IsRemoteDaemon() bool {
	if c.IsDemo() {
		return false
	}
	host := c.cli.DaemonHost()
	if host == "" {
		host = os.Getenv("DOCKER_HOST")
	}
	if host == "" {
		return false
	}
	// unix:///var/run/docker.sock or npipe:… = local
	if strings.HasPrefix(host, "unix://") || strings.HasPrefix(host, "npipe:") {
		return false
	}
	// tcp://, ssh://, http://, https:// = remote
	return true
}

// VolumeAccessMode describes how volume files are reached.
func (c *Client) VolumeAccessMode(ctx context.Context, name string) string {
	if _, ok := c.VolumeHostAccessible(ctx, name); ok {
		return "via host"
	}
	return "via docker"
}

// VolumeMountpoint returns the host mountpoint for a named volume.
func (c *Client) VolumeMountpoint(ctx context.Context, name string) (string, error) {
	if c.IsDemo() {
		return "/var/lib/docker/volumes/" + name + "/_data", nil
	}
	info, err := c.cli.VolumeInspect(ctx, name)
	if err != nil {
		return "", err
	}
	return info.Mountpoint, nil
}

// VolumeHostAccessible reports whether the volume mountpoint can be listed as this user.
// Always false for remote daemons (avoids writing to a coincidental local path).
func (c *Client) VolumeHostAccessible(ctx context.Context, name string) (string, bool) {
	if c.IsDemo() {
		return "", false
	}
	if c.IsRemoteDaemon() {
		return "", false
	}
	mp, err := c.VolumeMountpoint(ctx, name)
	if err != nil || mp == "" {
		return "", false
	}
	if _, err := os.ReadDir(mp); err != nil {
		return mp, false
	}
	return mp, true
}

// ListVolumeDir lists entries in a volume directory (path relative to volume root).
func (c *Client) ListVolumeDir(ctx context.Context, volumeName, rel string) ([]VolumeEntry, error) {
	rel = CleanVolPath(rel)
	if c.IsDemo() {
		if rel == "" {
			return DemoVolumeEntries(), nil
		}
		if rel == "base" || rel == "global" || rel == "pg_wal" || rel == "docker-entrypoint-initdb.d" {
			return []VolumeEntry{
				{Name: ".keep", Path: path.Join(rel, ".keep"), IsDir: false, Size: 0},
			}, nil
		}
		return nil, fmt.Errorf("not a directory")
	}
	if mp, ok := c.VolumeHostAccessible(ctx, volumeName); ok {
		return listHostDir(mp, rel)
	}
	return c.listVolumeDirDocker(ctx, volumeName, rel)
}

// ReadVolumeFile reads a file from a volume (path relative to volume root).
func (c *Client) ReadVolumeFile(ctx context.Context, volumeName, rel string) ([]byte, error) {
	rel = CleanVolPath(rel)
	if rel == "" || strings.HasSuffix(rel, "/") {
		return nil, fmt.Errorf("not a file path")
	}
	if c.IsDemo() {
		return []byte(DemoVolumeFile(rel)), nil
	}
	if mp, ok := c.VolumeHostAccessible(ctx, volumeName); ok {
		full, err := resolveContainedHostPath(mp, rel)
		if err != nil {
			return nil, err
		}
		return os.ReadFile(full)
	}
	return c.readVolumeFileDocker(ctx, volumeName, rel)
}

// WriteVolumeFile writes a file into a volume.
func (c *Client) WriteVolumeFile(ctx context.Context, volumeName, rel string, data []byte) error {
	if c.IsDemo() {
		return c.demoGuard()
	}
	rel = CleanVolPath(rel)
	if rel == "" {
		return fmt.Errorf("empty path")
	}
	if mp, ok := c.VolumeHostAccessible(ctx, volumeName); ok {
		full, err := resolveContainedHostPath(mp, rel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		// Re-check after mkdir in case a symlink race appeared under the mount.
		full, err = resolveContainedHostPath(mp, rel)
		if err != nil {
			return err
		}
		return os.WriteFile(full, data, 0o644)
	}
	return c.writeVolumeFileDocker(ctx, volumeName, rel, data)
}

// HostPathForVolumeFile returns a host path if the volume is directly readable
// and the resolved path stays under the mountpoint (no symlink escape).
func (c *Client) HostPathForVolumeFile(ctx context.Context, volumeName, rel string) (string, bool) {
	rel = CleanVolPath(rel)
	mp, ok := c.VolumeHostAccessible(ctx, volumeName)
	if !ok {
		return "", false
	}
	full, err := resolveContainedHostPath(mp, rel)
	if err != nil {
		return "", false
	}
	return full, true
}

// CleanVolPath normalizes a path relative to the volume root.
func CleanVolPath(rel string) string {
	rel = strings.TrimSpace(rel)
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(rel, "/")
	rel = path.Clean("/" + rel)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "." {
		return ""
	}
	return rel
}

// resolveContainedHostPath joins mountpoint+rel and ensures the result (after
// evaluating existing symlinks) stays under the mountpoint.
func resolveContainedHostPath(mountpoint, rel string) (string, error) {
	mp, err := absEvalPath(mountpoint)
	if err != nil {
		return "", err
	}
	rel = CleanVolPath(rel)
	target := mp
	if rel != "" {
		target = filepath.Join(mp, filepath.FromSlash(rel))
	}
	resolved, err := resolveExistingPrefix(mp, target)
	if err != nil {
		return "", err
	}
	if !pathIsUnder(mp, resolved) {
		return "", fmt.Errorf("path escapes volume mountpoint")
	}
	return resolved, nil
}

func absEvalPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if eval, err := filepath.EvalSymlinks(abs); err == nil {
		abs = eval
	}
	return filepath.Clean(abs), nil
}

// resolveExistingPrefix EvalSymlinks the longest existing prefix of target,
// then re-appends any missing trailing components.
func resolveExistingPrefix(root, target string) (string, error) {
	target = filepath.Clean(target)
	cur := target
	var missing []string
	for {
		if _, err := os.Lstat(cur); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if cur == root || filepath.Dir(cur) == cur {
			return "", fmt.Errorf("path outside volume mountpoint")
		}
		missing = append([]string{filepath.Base(cur)}, missing...)
		cur = filepath.Dir(cur)
	}
	resolved, err := filepath.EvalSymlinks(cur)
	if err != nil {
		return "", err
	}
	resolved = filepath.Clean(resolved)
	for _, part := range missing {
		if part == ".." {
			return "", fmt.Errorf("path escapes volume mountpoint")
		}
		resolved = filepath.Join(resolved, part)
	}
	return filepath.Clean(resolved), nil
}

func pathIsUnder(root, p string) bool {
	root = filepath.Clean(root)
	p = filepath.Clean(p)
	if p == root {
		return true
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func listHostDir(mountpoint, rel string) ([]VolumeEntry, error) {
	dir, err := resolveContainedHostPath(mountpoint, rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]VolumeEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := e.Name()
		child := name
		if rel != "" {
			child = path.Join(rel, name)
		}
		out = append(out, VolumeEntry{
			Name:  name,
			Path:  child,
			IsDir: e.IsDir(),
			Size:  info.Size(),
		})
	}
	sortVolumeEntries(out)
	return out, nil
}

func (c *Client) ensureBusybox(ctx context.Context) error {
	img := volumeHelperImage()
	_, err := c.cli.ImageInspect(ctx, img)
	if err == nil {
		return nil
	}
	rc, err := c.cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull %s failed — set DOCKAFE_BUSYBOX_IMAGE or pre-load the image: %w", img, err)
	}
	defer rc.Close()
	_, _ = io.Copy(io.Discard, rc)
	// Re-inspect to ensure pull actually produced the image
	if _, err := c.cli.ImageInspect(ctx, img); err != nil {
		return fmt.Errorf("pull %s failed — set DOCKAFE_BUSYBOX_IMAGE or pre-load the image: %w", img, err)
	}
	return nil
}

func (c *Client) runVolumeCmd(ctx context.Context, volumeName string, rw bool, cmd []string) (string, error) {
	if err := validateVolumeName(volumeName); err != nil {
		return "", err
	}
	if err := c.ensureBusybox(ctx); err != nil {
		return "", err
	}
	img := volumeHelperImage()
	mode := "ro"
	if rw {
		mode = "rw"
	}
	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image:      img,
		Cmd:        cmd,
		Tty:        false,
		Entrypoint: []string{},
	}, &container.HostConfig{
		Binds:      []string{volumeName + ":/vol:" + mode},
		AutoRemove: false,
	}, nil, nil, "")
	if err != nil {
		return "", err
	}
	id := resp.ID
	defer func() {
		_ = c.cli.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
	}()

	if err := c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return "", err
	}

	var statusCode int64
	statusCh, errCh := c.cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case st := <-statusCh:
		statusCode = st.StatusCode
	case <-ctx.Done():
		return "", ctx.Err()
	}

	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", err
	}
	defer rc.Close()
	out, err := demuxDockerLogs(rc)
	if err != nil {
		return "", err
	}
	if statusCode != 0 {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = fmt.Sprintf("exit code %d", statusCode)
		}
		return out, fmt.Errorf("volume helper: %s", msg)
	}
	return out, nil
}

func (c *Client) listVolumeDirDocker(ctx context.Context, volumeName, rel string) ([]VolumeEntry, error) {
	target := "/vol"
	if rel != "" {
		target = path.Join("/vol", rel)
	}
	script := fmt.Sprintf(`cd %s 2>/dev/null || exit 2; ls -1ap`, ShellQuote(target))
	out, err := c.runVolumeCmd(ctx, volumeName, false, []string{"sh", "-c", script})
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	entries := make([]VolumeEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "./" || line == "../" || line == "." || line == ".." {
			continue
		}
		isDir := strings.HasSuffix(line, "/")
		name := strings.TrimSuffix(line, "/")
		child := name
		if rel != "" {
			child = path.Join(rel, name)
		}
		entries = append(entries, VolumeEntry{
			Name:  name,
			Path:  child,
			IsDir: isDir,
		})
	}
	sortVolumeEntries(entries)
	return entries, nil
}

func (c *Client) readVolumeFileDocker(ctx context.Context, volumeName, rel string) ([]byte, error) {
	out, err := c.runVolumeCmd(ctx, volumeName, false, []string{"cat", path.Join("/vol", rel)})
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

func (c *Client) writeVolumeFileDocker(ctx context.Context, volumeName, rel string, data []byte) error {
	if err := validateVolumeName(volumeName); err != nil {
		return err
	}
	if err := c.ensureBusybox(ctx); err != nil {
		return err
	}
	img := volumeHelperImage()
	parent := path.Dir(path.Join("/vol", rel))
	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image:      img,
		Cmd:        []string{"sh", "-c", "mkdir -p " + ShellQuote(parent) + " && sleep 60"},
		Entrypoint: []string{},
	}, &container.HostConfig{
		Binds: []string{volumeName + ":/vol:rw"},
	}, nil, nil, "")
	if err != nil {
		return err
	}
	id := resp.ID
	defer func() {
		_ = c.cli.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
	}()
	if err := c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return err
	}
	// Brief wait for mkdir
	time.Sleep(200 * time.Millisecond)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name:    path.Join("vol", rel),
		Mode:    0o644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return c.cli.CopyToContainer(ctx, id, "/", &buf, container.CopyToContainerOptions{})
}

func sortVolumeEntries(entries []VolumeEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

// ShellQuote wraps s for safe use inside single-quoted shell strings.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// validateVolumeName rejects names that would break Docker bind syntax (":").
func validateVolumeName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("empty volume name")
	}
	if strings.ContainsAny(name, ":/") {
		return fmt.Errorf("invalid volume name")
	}
	return nil
}
