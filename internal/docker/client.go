package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

const composeProjectLabel = "com.docker.compose.project"
const composeServiceLabel = "com.docker.compose.service"

type Client struct {
	cli *client.Client
}

func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

type ContainerInfo struct {
	ID       string
	Name     string
	Image    string
	Status   string
	State    string
	Project  string
	Service  string
	Ports    string
	CPU      string
	Mem      string
	CPUVal   float64
	MemBytes uint64
	Created  time.Time
	Running  bool
}

type ComposeGroup struct {
	Name       string
	Containers []ContainerInfo
	Running    int
	Total      int
	CPU        string
	Mem        string
	Ports      string
	CPUVal     float64
	MemBytes   uint64
}

type ImageInfo struct {
	ID        string
	FullID    string
	Tags      string
	Size      string
	SizeBytes int64
	Created   time.Time
}

type VolumeInfo struct {
	Name       string
	Driver     string
	Mountpoint string
	Scope      string
	Created    time.Time
	InUse      bool
	RefCount   int
	UsedBy     string // container names, comma-separated
}

type NetworkInfo struct {
	ID     string
	Name   string
	Driver string
	Scope  string
	Subnet string
}

func (c *Client) ListContainers(ctx context.Context, withStats bool) ([]ContainerInfo, error) {
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	out := make([]ContainerInfo, 0, len(list))
	for _, item := range list {
		name := containerName(item.Names)
		project := item.Labels[composeProjectLabel]
		service := item.Labels[composeServiceLabel]
		info := ContainerInfo{
			ID:      item.ID,
			Name:    name,
			Image:   item.Image,
			Status:  item.Status,
			State:   string(item.State),
			Project: project,
			Service: service,
			Ports:   formatPorts(item.Ports),
			CPU:     "-",
			Mem:     "-",
			Created: time.Unix(item.Created, 0),
			Running: item.State == container.StateRunning,
		}
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		return out[i].Name < out[j].Name
	})

	if withStats {
		c.fillStats(ctx, out)
	}
	return out, nil
}

func (c *Client) ListComposeGroups(ctx context.Context, withStats bool) ([]ComposeGroup, error) {
	containers, err := c.ListContainers(ctx, withStats)
	if err != nil {
		return nil, err
	}

	groups := map[string]*ComposeGroup{}
	order := []string{}

	for _, ctr := range containers {
		key := ctr.Project
		if key == "" {
			key = "(standalone)"
		}
		g, ok := groups[key]
		if !ok {
			g = &ComposeGroup{Name: key}
			groups[key] = g
			order = append(order, key)
		}
		g.Containers = append(g.Containers, ctr)
		g.Total++
		if ctr.Running {
			g.Running++
		}
	}

	out := make([]ComposeGroup, 0, len(order))
	for _, name := range order {
		g := groups[name]
		g.Ports = uniquePorts(g.Containers)
		g.CPU, g.Mem = aggregateUsage(g.Containers)
		g.CPUVal, g.MemBytes = aggregateUsageVals(g.Containers)
		out = append(out, *g)
	}
	return out, nil
}

func (c *Client) ListImages(ctx context.Context) ([]ImageInfo, error) {
	list, err := c.cli.ImageList(ctx, image.ListOptions{All: false})
	if err != nil {
		return nil, err
	}
	out := make([]ImageInfo, 0, len(list))
	for _, img := range list {
		tags := strings.Join(img.RepoTags, ", ")
		if tags == "" {
			tags = "<none>"
		}
		out = append(out, ImageInfo{
			ID:        shortID(img.ID),
			FullID:    img.ID,
			Tags:      tags,
			Size:      formatBytes(img.Size),
			SizeBytes: img.Size,
			Created:   time.Unix(img.Created, 0),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Created.After(out[j].Created)
	})
	return out, nil
}

func (c *Client) ListVolumes(ctx context.Context) ([]VolumeInfo, error) {
	resp, err := c.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}
	usage := c.volumeUsageMap(ctx)
	out := make([]VolumeInfo, 0, len(resp.Volumes))
	for _, vol := range resp.Volumes {
		created, _ := time.Parse(time.RFC3339Nano, vol.CreatedAt)
		users := usage[vol.Name]
		info := VolumeInfo{
			Name:       vol.Name,
			Driver:     vol.Driver,
			Mountpoint: vol.Mountpoint,
			Scope:      vol.Scope,
			Created:    created,
			RefCount:   len(users),
			InUse:      len(users) > 0,
		}
		if len(users) > 0 {
			info.UsedBy = strings.Join(users, ", ")
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].InUse != out[j].InUse {
			return out[i].InUse
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// volumeUsageMap maps volume name → container names currently mounting it.
func (c *Client) volumeUsageMap(ctx context.Context) map[string][]string {
	out := map[string][]string{}
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return out
	}
	for _, item := range list {
		cname := containerName(item.Names)
		for _, m := range item.Mounts {
			if m.Type != "volume" || m.Name == "" {
				continue
			}
			out[m.Name] = appendUnique(out[m.Name], cname)
		}
	}
	return out
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

func (c *Client) ListNetworks(ctx context.Context) ([]NetworkInfo, error) {
	list, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]NetworkInfo, 0, len(list))
	for _, net := range list {
		subnet := ""
		if net.IPAM.Config != nil {
			parts := make([]string, 0, len(net.IPAM.Config))
			for _, cfg := range net.IPAM.Config {
				if cfg.Subnet != "" {
					parts = append(parts, cfg.Subnet)
				}
			}
			subnet = strings.Join(parts, ", ")
		}
		out = append(out, NetworkInfo{
			ID:     shortID(net.ID),
			Name:   net.Name,
			Driver: net.Driver,
			Scope:  net.Scope,
			Subnet: subnet,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force, RemoveVolumes: false})
}

func (c *Client) StartCompose(ctx context.Context, project string) error {
	ids, err := c.projectContainerIDs(ctx, project)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := c.StartContainer(ctx, id); err != nil {
			return fmt.Errorf("%s: %w", shortID(id), err)
		}
	}
	return nil
}

func (c *Client) StopCompose(ctx context.Context, project string) error {
	ids, err := c.projectContainerIDs(ctx, project)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := c.StopContainer(ctx, id); err != nil {
			return fmt.Errorf("%s: %w", shortID(id), err)
		}
	}
	return nil
}

func (c *Client) RestartCompose(ctx context.Context, project string) error {
	ids, err := c.projectContainerIDs(ctx, project)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := c.RestartContainer(ctx, id); err != nil {
			return fmt.Errorf("%s: %w", shortID(id), err)
		}
	}
	return nil
}

func (c *Client) RemoveCompose(ctx context.Context, project string, force bool) error {
	ids, err := c.projectContainerIDs(ctx, project)
	if err != nil {
		return err
	}
	for _, id := range ids {
		_ = c.StopContainer(ctx, id)
		if err := c.RemoveContainer(ctx, id, force); err != nil {
			return fmt.Errorf("%s: %w", shortID(id), err)
		}
	}
	return nil
}

func (c *Client) RebuildCompose(ctx context.Context, project string) error {
	if project == "" || project == "(standalone)" {
		return fmt.Errorf("rebuild only works for compose projects")
	}
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", project, "up", "-d", "--build")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	_, err := c.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: force, PruneChildren: true})
	return err
}

func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	return c.cli.VolumeRemove(ctx, name, force)
}

func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	return c.cli.NetworkRemove(ctx, id)
}

func (c *Client) projectContainerIDs(ctx context.Context, project string) ([]string, error) {
	if project == "" || project == "(standalone)" {
		return nil, fmt.Errorf("invalid compose project")
	}
	f := filters.NewArgs()
	f.Add("label", composeProjectLabel+"="+project)
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.ID)
	}
	return ids, nil
}

func (c *Client) fillStats(ctx context.Context, containers []ContainerInfo) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range containers {
		if !containers[i].Running {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			statsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			cpu, mem, cpuVal, memBytes, err := c.oneShotStats(statsCtx, containers[idx].ID)
			if err != nil {
				return
			}
			containers[idx].CPU = cpu
			containers[idx].Mem = mem
			containers[idx].CPUVal = cpuVal
			containers[idx].MemBytes = memBytes
		}(i)
	}
	wg.Wait()
}

func (c *Client) oneShotStats(ctx context.Context, id string) (string, string, float64, uint64, error) {
	resp, err := c.cli.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return "", "", 0, 0, err
	}
	defer resp.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil && err != io.EOF {
		return "", "", 0, 0, err
	}

	cpuVal := calcCPU(stats)
	cpu := fmt.Sprintf("%.1f%%", cpuVal)
	mem := formatMem(stats.MemoryStats.Usage, stats.MemoryStats.Limit)
	return cpu, mem, cpuVal, stats.MemoryStats.Usage, nil
}

func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

func formatPorts(ports []container.Port) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ports))
	seen := map[string]struct{}{}
	for _, p := range ports {
		var s string
		if p.PublicPort > 0 {
			ip := p.IP
			if ip == "" || ip == "0.0.0.0" {
				ip = "0.0.0.0"
			}
			s = fmt.Sprintf("%s:%d→%d/%s", ip, p.PublicPort, p.PrivatePort, p.Type)
		} else {
			s = fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		parts = append(parts, s)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func uniquePorts(containers []ContainerInfo) string {
	seen := map[string]struct{}{}
	parts := []string{}
	for _, c := range containers {
		if c.Ports == "-" || c.Ports == "" {
			continue
		}
		for _, p := range strings.Split(c.Ports, ", ") {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func aggregateUsage(containers []ContainerInfo) (string, string) {
	var cpuSum float64
	var memSum uint64
	var memLimit uint64
	cpuCount := 0
	memCount := 0

	for _, c := range containers {
		if c.CPU != "-" && c.CPU != "" {
			var v float64
			if _, err := fmt.Sscanf(c.CPU, "%f%%", &v); err == nil {
				cpuSum += v
				cpuCount++
			}
		}
		if c.Mem != "-" && c.Mem != "" {
			// format: "12.3MiB / 1.9GiB"
			parts := strings.Split(c.Mem, " / ")
			if len(parts) == 2 {
				if u, ok := parseHumanBytes(parts[0]); ok {
					memSum += u
					memCount++
				}
				if l, ok := parseHumanBytes(parts[1]); ok && l > memLimit {
					memLimit = l
				}
			}
		}
	}

	cpu := "-"
	mem := "-"
	if cpuCount > 0 {
		cpu = fmt.Sprintf("%.1f%%", cpuSum)
	}
	if memCount > 0 {
		if memLimit > 0 {
			mem = fmt.Sprintf("%s / %s", formatBytes(int64(memSum)), formatBytes(int64(memLimit)))
		} else {
			mem = formatBytes(int64(memSum))
		}
	}
	return cpu, mem
}

func aggregateUsageVals(containers []ContainerInfo) (float64, uint64) {
	var cpuSum float64
	var memSum uint64
	for _, c := range containers {
		cpuSum += c.CPUVal
		memSum += c.MemBytes
	}
	return cpuSum, memSum
}

func calcCPU(stats container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	online := float64(stats.CPUStats.OnlineCPUs)
	if online == 0 {
		online = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if systemDelta > 0 && cpuDelta > 0 && online > 0 {
		return (cpuDelta / systemDelta) * online * 100.0
	}
	return 0
}

func formatCPU(stats container.StatsResponse) string {
	return fmt.Sprintf("%.1f%%", calcCPU(stats))
}

func formatMem(usage, limit uint64) string {
	if limit == 0 {
		return formatBytes(int64(usage))
	}
	return fmt.Sprintf("%s / %s", formatBytes(int64(usage)), formatBytes(int64(limit)))
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func parseHumanBytes(s string) (uint64, bool) {
	s = strings.TrimSpace(s)
	var val float64
	var unit string
	n, err := fmt.Sscanf(s, "%f%s", &val, &unit)
	if err != nil || n < 1 {
		return 0, false
	}
	mult := float64(1)
	switch strings.ToUpper(unit) {
	case "B", "":
		mult = 1
	case "KIB", "KB", "K":
		mult = 1024
	case "MIB", "MB", "M":
		mult = 1024 * 1024
	case "GIB", "GB", "G":
		mult = 1024 * 1024 * 1024
	case "TIB", "TB", "T":
		mult = 1024 * 1024 * 1024 * 1024
	}
	return uint64(val * mult), true
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
