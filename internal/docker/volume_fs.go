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
	info, err := c.cli.VolumeInspect(ctx, name)
	if err != nil {
		return "", err
	}
	return info.Mountpoint, nil
}

// VolumeHostAccessible reports whether the volume mountpoint can be listed as this user.
// Always false for remote daemons (avoids writing to a coincidental local path).
func (c *Client) VolumeHostAccessible(ctx context.Context, name string) (string, bool) {
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
	if mp, ok := c.VolumeHostAccessible(ctx, volumeName); ok {
		return os.ReadFile(filepath.Join(mp, filepath.FromSlash(rel)))
	}
	return c.readVolumeFileDocker(ctx, volumeName, rel)
}

// WriteVolumeFile writes a file into a volume.
func (c *Client) WriteVolumeFile(ctx context.Context, volumeName, rel string, data []byte) error {
	rel = CleanVolPath(rel)
	if rel == "" {
		return fmt.Errorf("empty path")
	}
	if mp, ok := c.VolumeHostAccessible(ctx, volumeName); ok {
		full := filepath.Join(mp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		return os.WriteFile(full, data, 0o644)
	}
	return c.writeVolumeFileDocker(ctx, volumeName, rel, data)
}

// HostPathForVolumeFile returns a host path if the volume is directly readable.
func (c *Client) HostPathForVolumeFile(ctx context.Context, volumeName, rel string) (string, bool) {
	rel = CleanVolPath(rel)
	mp, ok := c.VolumeHostAccessible(ctx, volumeName)
	if !ok {
		return "", false
	}
	return filepath.Join(mp, filepath.FromSlash(rel)), true
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

func listHostDir(mountpoint, rel string) ([]VolumeEntry, error) {
	dir := mountpoint
	if rel != "" {
		dir = filepath.Join(mountpoint, filepath.FromSlash(rel))
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
	if err := c.ensureBusybox(ctx); err != nil {
		return "", err
	}
	img := volumeHelperImage()
	mode := "ro"
	if rw {
		mode = "rw"
	}
	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image: img,
		Cmd:   cmd,
		Tty:   false,
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
	if err := c.ensureBusybox(ctx); err != nil {
		return err
	}
	img := volumeHelperImage()
	parent := path.Dir(path.Join("/vol", rel))
	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image: img,
		Cmd:   []string{"sh", "-c", "mkdir -p " + ShellQuote(parent) + " && sleep 60"},
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
