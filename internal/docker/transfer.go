package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
)

// ExportVolumePathToLocal copies a volume file/dir to a local path.
// Directories are written as a tar archive (Docker copy format).
func (c *Client) ExportVolumePathToLocal(ctx context.Context, volumeName, rel, localPath string, isDir bool) error {
	if err := validateVolumeName(volumeName); err != nil {
		return err
	}
	rel = CleanVolPath(rel)
	localPath = filepath.Clean(localPath)
	if localPath == "" || localPath == "." {
		return fmt.Errorf("empty local path")
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	if isDir && filepath.Ext(localPath) == "" {
		localPath += ".tar"
	}

	if !isDir {
		data, err := c.ReadVolumeFile(ctx, volumeName, rel)
		if err != nil {
			return err
		}
		return os.WriteFile(localPath, data, 0o644)
	}

	if c.IsDemo() {
		return fmt.Errorf("demo mode cannot export directories")
	}
	if err := c.ensureBusybox(ctx); err != nil {
		return err
	}
	img := volumeHelperImage()
	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image:      img,
		Cmd:        []string{"sleep", "30"},
		Entrypoint: []string{},
	}, &container.HostConfig{
		Binds: []string{volumeName + ":/vol:ro"},
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
	time.Sleep(100 * time.Millisecond)

	src := "/vol"
	if rel != "" {
		src = path.Join("/vol", rel)
	}
	rc, _, err := c.cli.CopyFromContainer(ctx, id, src)
	if err != nil {
		return err
	}
	defer rc.Close()

	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, rc)
	return err
}

// ImportLocalFileToVolume copies a local file into the volume at rel.
func (c *Client) ImportLocalFileToVolume(ctx context.Context, volumeName, rel, localPath string) error {
	if err := validateVolumeName(volumeName); err != nil {
		return err
	}
	rel = CleanVolPath(rel)
	if rel == "" {
		return fmt.Errorf("empty volume path")
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	return c.WriteVolumeFile(ctx, volumeName, rel, data)
}

// RemoveVolumePath deletes a file or directory inside a volume.
func (c *Client) RemoveVolumePath(ctx context.Context, volumeName, rel string) error {
	if err := validateVolumeName(volumeName); err != nil {
		return err
	}
	rel = CleanVolPath(rel)
	if rel == "" {
		return fmt.Errorf("refusing to delete volume root")
	}
	target := path.Join("/vol", rel)
	_, err := c.runVolumeCmd(ctx, volumeName, true, []string{"rm", "-rf", target})
	return err
}

// SuggestLocalExportPath builds a default download path for a volume entry.
func SuggestLocalExportPath(volumeName, rel string, isDir bool) string {
	base := filepath.Base(rel)
	if base == "" || base == "." {
		base = volumeName
	}
	if isDir {
		base += ".tar"
	}
	return filepath.Join(DefaultDownloadDir(), volumeName, base)
}
