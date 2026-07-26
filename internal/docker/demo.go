package docker

import (
	"context"
	"fmt"
	"time"
)

// NewDemo returns a client that serves canned sample data (no Docker socket).
func NewDemo() *Client {
	return &Client{demo: true}
}

// IsDemo reports whether this client serves fake sample data.
func (c *Client) IsDemo() bool {
	return c != nil && c.demo
}

func (c *Client) demoGuard() error {
	if c.IsDemo() {
		return fmt.Errorf("demo mode is read-only")
	}
	return nil
}

func demoContainers() []ContainerInfo {
	now := time.Now().Add(-48 * time.Hour)
	return []ContainerInfo{
		ctr("a1b2c3d4e5f6", "web", "shop-api", "web", "nginx:1.27-alpine", "Up 2 hours", true, "0.0.0.0:8080->80/tcp", "0.4%", "128.0MiB / 7.8GiB", 0.4, 134217728, now),
		ctr("b2c3d4e5f6a7", "api", "shop-api", "api", "shop-api:1.2.0", "Up 2 hours", true, "0.0.0.0:3000->3000/tcp", "1.2%", "256.4MiB / 7.8GiB", 1.2, 268435456, now),
		ctr("c3d4e5f6a7b8", "db", "shop-api", "db", "postgres:16-alpine", "Up 2 hours", true, "127.0.0.1:5432->5432/tcp", "0.8%", "312.1MiB / 7.8GiB", 0.8, 327155712, now),
		ctr("d4e5f6a7b8c9", "cache", "shop-api", "cache", "redis:7-alpine", "Up 2 hours", true, "127.0.0.1:6379->6379/tcp", "0.1%", "24.5MiB / 7.8GiB", 0.1, 25690112, now),
		ctr("e5f6a7b8c9d0", "grafana", "observability", "grafana", "grafana/grafana:11.0.0", "Up 5 hours", true, "0.0.0.0:3001->3000/tcp", "0.3%", "189.2MiB / 7.8GiB", 0.3, 198180864, now),
		ctr("f6a7b8c9d0e1", "prometheus", "observability", "prometheus", "prom/prometheus:v2.53.0", "Up 5 hours", true, "0.0.0.0:9090->9090/tcp", "0.6%", "98.7MiB / 7.8GiB", 0.6, 103546880, now),
		ctr("a7b8c9d0e1f2", "loki", "observability", "loki", "grafana/loki:3.0.0", "Exited (0) 10 minutes ago", false, "", "-", "-", 0, 0, now),
		ctr("b8c9d0e1f2a3", "worker", "jobs", "worker", "jobs-worker:0.9.1", "Up 1 hour", true, "", "2.1%", "410.0MiB / 7.8GiB", 2.1, 429916160, now),
		ctr("c9d0e1f2a3b4", "beat", "jobs", "beat", "jobs-worker:0.9.1", "Up 1 hour", true, "", "0.2%", "64.0MiB / 7.8GiB", 0.2, 67108864, now),
		ctr("d0e1f2a3b4c5", "mailhog", "", "", "mailhog/mailhog:v1.0.1", "Up 3 hours", true, "0.0.0.0:8025->8025/tcp", "0.0%", "12.3MiB / 7.8GiB", 0.0, 12897484, now),
		ctr("e1f2a3b4c5d6", "old-redis", "legacy-stack", "redis", "redis:6-alpine", "Exited (1) 2 days ago", false, "", "-", "-", 0, 0, now.Add(-96*time.Hour)),
		ctr("f2a3b4c5d6e7", "old-api", "legacy-stack", "api", "legacy-api:0.3.0", "Exited (1) 2 days ago", false, "", "-", "-", 0, 0, now.Add(-96*time.Hour)),
	}
}

func ctr(id, name, project, service, image, status string, running bool, ports, cpu, mem string, cpuVal float64, memBytes uint64, created time.Time) ContainerInfo {
	state := "exited"
	if running {
		state = "running"
	}
	return ContainerInfo{
		ID:       id,
		Name:     name,
		Image:    image,
		Status:   status,
		State:    state,
		Project:  project,
		Service:  service,
		Ports:    ports,
		CPU:      cpu,
		Mem:      mem,
		CPUVal:   cpuVal,
		MemBytes: memBytes,
		Created:  created,
		Running:  running,
	}
}

func demoGroupsFrom(containers []ContainerInfo) []ComposeGroup {
	groups := map[string]*ComposeGroup{}
	order := []string{}
	for _, c := range containers {
		key := c.Project
		if key == "" {
			key = "(standalone)"
		}
		g, ok := groups[key]
		if !ok {
			g = &ComposeGroup{Name: key}
			groups[key] = g
			order = append(order, key)
		}
		g.Containers = append(g.Containers, c)
		g.Total++
		if c.Running {
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
	return out
}

func demoImages() []ImageInfo {
	now := time.Now()
	return []ImageInfo{
		{ID: "sha256:111", FullID: "sha256:111aaa", Tags: "nginx:1.27-alpine", Size: "42.0MB", SizeBytes: 42_000_000, Created: now.Add(-24 * time.Hour)},
		{ID: "sha256:222", FullID: "sha256:222bbb", Tags: "postgres:16-alpine", Size: "245MB", SizeBytes: 245_000_000, Created: now.Add(-48 * time.Hour)},
		{ID: "sha256:333", FullID: "sha256:333ccc", Tags: "redis:7-alpine", Size: "41.4MB", SizeBytes: 41_400_000, Created: now.Add(-72 * time.Hour)},
		{ID: "sha256:444", FullID: "sha256:444ddd", Tags: "shop-api:1.2.0", Size: "88.1MB", SizeBytes: 88_100_000, Created: now.Add(-6 * time.Hour)},
		{ID: "sha256:555", FullID: "sha256:555eee", Tags: "grafana/grafana:11.0.0", Size: "312MB", SizeBytes: 312_000_000, Created: now.Add(-120 * time.Hour)},
		{ID: "sha256:666", FullID: "sha256:666fff", Tags: "busybox:1.36.1", Size: "4.3MB", SizeBytes: 4_300_000, Created: now.Add(-200 * time.Hour)},
	}
}

func demoVolumes() []VolumeInfo {
	now := time.Now().Add(-72 * time.Hour)
	return []VolumeInfo{
		{Name: "shop-api_pgdata", Driver: "local", Mountpoint: "/var/lib/docker/volumes/shop-api_pgdata/_data", Scope: "local", Created: now, InUse: true, RefCount: 1, UsedBy: "db"},
		{Name: "shop-api_redis", Driver: "local", Mountpoint: "/var/lib/docker/volumes/shop-api_redis/_data", Scope: "local", Created: now, InUse: true, RefCount: 1, UsedBy: "cache"},
		{Name: "observability_grafana", Driver: "local", Mountpoint: "/var/lib/docker/volumes/observability_grafana/_data", Scope: "local", Created: now, InUse: true, RefCount: 1, UsedBy: "grafana"},
		{Name: "jobs_queue", Driver: "local", Mountpoint: "/var/lib/docker/volumes/jobs_queue/_data", Scope: "local", Created: now, InUse: true, RefCount: 1, UsedBy: "worker"},
		{Name: "orphan_backup", Driver: "local", Mountpoint: "/var/lib/docker/volumes/orphan_backup/_data", Scope: "local", Created: now.Add(-240 * time.Hour), InUse: false, RefCount: 0},
		{Name: "legacy-stack_data", Driver: "local", Mountpoint: "/var/lib/docker/volumes/legacy-stack_data/_data", Scope: "local", Created: now.Add(-400 * time.Hour), InUse: false, RefCount: 0},
	}
}

func demoNetworks() []NetworkInfo {
	return []NetworkInfo{
		{ID: "net111", Name: "bridge", Driver: "bridge", Scope: "local", Subnet: "172.17.0.0/16"},
		{ID: "net222", Name: "host", Driver: "host", Scope: "local", Subnet: ""},
		{ID: "net333", Name: "shop-api_default", Driver: "bridge", Scope: "local", Subnet: "172.20.0.0/16"},
		{ID: "net444", Name: "observability_default", Driver: "bridge", Scope: "local", Subnet: "172.21.0.0/16"},
		{ID: "net555", Name: "jobs_default", Driver: "bridge", Scope: "local", Subnet: "172.22.0.0/16"},
	}
}

// DemoVolumeEntries returns a sample file tree for screenshot / demo browsing.
func DemoVolumeEntries() []VolumeEntry {
	return []VolumeEntry{
		{Name: "postgresql.conf", Path: "postgresql.conf", IsDir: false, Size: 28400},
		{Name: "pg_hba.conf", Path: "pg_hba.conf", IsDir: false, Size: 4800},
		{Name: "base", Path: "base", IsDir: true},
		{Name: "global", Path: "global", IsDir: true},
		{Name: "pg_wal", Path: "pg_wal", IsDir: true},
		{Name: "docker-entrypoint-initdb.d", Path: "docker-entrypoint-initdb.d", IsDir: true},
	}
}

func DemoVolumeFile(rel string) string {
	switch CleanVolPath(rel) {
	case "postgresql.conf":
		return `# demo postgresql.conf — sample only
listen_addresses = '*'
max_connections = 100
shared_buffers = 128MB
work_mem = 4MB
log_timezone = 'UTC'
datestyle = 'iso, mdy'
`
	case "pg_hba.conf":
		return `# TYPE  DATABASE  USER  ADDRESS       METHOD
local   all       all                 trust
host    all       all   127.0.0.1/32  scram-sha-256
host    all       all   0.0.0.0/0     scram-sha-256
`
	default:
		return "# demo file\n"
	}
}

func (c *Client) listDemoContainers(_ context.Context, _ bool) ([]ContainerInfo, error) {
	return demoContainers(), nil
}

func (c *Client) listDemoComposeGroups(ctx context.Context, withStats bool) ([]ComposeGroup, error) {
	ctrs, err := c.listDemoContainers(ctx, withStats)
	if err != nil {
		return nil, err
	}
	return demoGroupsFrom(ctrs), nil
}

func (c *Client) listDemoImages(context.Context) ([]ImageInfo, error) {
	return demoImages(), nil
}

func (c *Client) listDemoVolumes(context.Context) ([]VolumeInfo, error) {
	return demoVolumes(), nil
}

func (c *Client) listDemoNetworks(context.Context) ([]NetworkInfo, error) {
	return demoNetworks(), nil
}

func (c *Client) demoSystemInfo(context.Context) (string, error) {
	return "Docker 29.0.0 · Demo Linux/x86_64 · containers 12 (running 8) · images 6 · CPUs 8 · Mem 7.8GiB", nil
}
