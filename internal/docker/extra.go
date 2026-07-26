package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
)

func (c *Client) InspectContainer(ctx context.Context, id string) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Client) InspectImage(ctx context.Context, id string) (string, error) {
	info, _, err := c.cli.ImageInspectWithRaw(ctx, id)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Client) InspectVolume(ctx context.Context, name string) (string, error) {
	info, err := c.cli.VolumeInspect(ctx, name)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Client) InspectNetwork(ctx context.Context, id string) (string, error) {
	info, err := c.cli.NetworkInspect(ctx, id, network.InspectOptions{})
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Client) ContainerLogs(ctx context.Context, id string, tail string, timestamps bool) (string, error) {
	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: timestamps,
		Tail:       tail,
	})
	if err != nil {
		return "", err
	}
	defer rc.Close()
	return demuxDockerLogs(rc)
}

func (c *Client) ContainerTop(ctx context.Context, id string) (string, error) {
	top, err := c.cli.ContainerTop(ctx, id, []string{})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(strings.Join(top.Titles, "\t"))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("-", 72))
	b.WriteByte('\n')
	for _, proc := range top.Processes {
		b.WriteString(strings.Join(proc, "\t"))
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func (c *Client) PauseContainer(ctx context.Context, id string) error {
	return c.cli.ContainerPause(ctx, id)
}

func (c *Client) UnpauseContainer(ctx context.Context, id string) error {
	return c.cli.ContainerUnpause(ctx, id)
}

func (c *Client) KillContainer(ctx context.Context, id string) error {
	return c.cli.ContainerKill(ctx, id, "SIGKILL")
}

func (c *Client) PruneImages(ctx context.Context) (string, error) {
	report, err := c.cli.ImagesPrune(ctx, filters.NewArgs())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("removed %d images, reclaimed %s", len(report.ImagesDeleted), formatBytes(int64(report.SpaceReclaimed))), nil
}

func (c *Client) PruneVolumes(ctx context.Context) (string, error) {
	report, err := c.cli.VolumesPrune(ctx, filters.NewArgs())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("removed %d volumes, reclaimed %s", len(report.VolumesDeleted), formatBytes(int64(report.SpaceReclaimed))), nil
}

func (c *Client) PruneNetworks(ctx context.Context) (string, error) {
	report, err := c.cli.NetworksPrune(ctx, filters.NewArgs())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("removed %d networks", len(report.NetworksDeleted)), nil
}

func (c *Client) PruneContainers(ctx context.Context) (string, error) {
	report, err := c.cli.ContainersPrune(ctx, filters.NewArgs())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("removed %d containers, reclaimed %s", len(report.ContainersDeleted), formatBytes(int64(report.SpaceReclaimed))), nil
}

func (c *Client) SystemInfo(ctx context.Context) (string, error) {
	if c.IsDemo() {
		return c.demoSystemInfo(ctx)
	}
	info, err := c.cli.Info(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"host %s · Docker %s · %s/%s · containers %d (running %d) · images %d · CPUs %d · Mem %s",
		c.Host(),
		info.ServerVersion,
		info.OperatingSystem,
		info.Architecture,
		info.Containers,
		info.ContainersRunning,
		info.Images,
		info.NCPU,
		formatBytes(info.MemTotal),
	), nil
}

// demuxDockerLogs strips the 8-byte docker multiplex header when present.
func demuxDockerLogs(r io.Reader) (string, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	if len(raw) == 0 {
		return "(no logs)", nil
	}

	var stdout, stderr bytes.Buffer
	_, copyErr := stdcopy.StdCopy(&stdout, &stderr, bytes.NewReader(raw))
	if copyErr == nil || stdout.Len()+stderr.Len() > 0 {
		out := stdout.String()
		if stderr.Len() > 0 {
			if out != "" && !strings.HasSuffix(out, "\n") {
				out += "\n"
			}
			out += stderr.String()
		}
		if strings.TrimSpace(out) != "" {
			return sanitizeLogText(out), nil
		}
	}

	// TTY / non-multiplexed fallback
	return sanitizeLogText(string(raw)), nil
}

func sanitizeLogText(s string) string {
	// Keep ANSI color codes; only normalize newlines and drop lone CRs.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if strings.TrimSpace(s) == "" {
		return "(no logs)"
	}
	return s
}
