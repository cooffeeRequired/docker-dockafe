package docker

import (
	"bufio"
	"context"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// HostMetrics is a point-in-time sample of host / docker-pressure resources.
type HostMetrics struct {
	HostCPUPct   float64 // 0–100; -1 if unavailable
	HostMemPct   float64 // 0–100; -1 if unavailable
	HostMemUsed  uint64
	HostMemTotal uint64
	NCPU         int
	DiskUsedPct  float64 // 0–100; -1 if unavailable
	DockerCPUPct float64 // sum of running containers
	DockerMem    uint64
	DockerN      int
	Source       string // "local" | "docker" | "demo"
}

type cpuSample struct {
	idle  uint64
	total uint64
}

var (
	hostCPUMu   sync.Mutex
	hostCPUPrev cpuSample
	hostCPUInit bool
)

// SampleHostMetrics gathers host CPU/RAM/disk plus aggregate docker container load.
func (c *Client) SampleHostMetrics(ctx context.Context) (HostMetrics, error) {
	if c.IsDemo() {
		return demoHostMetrics(), nil
	}

	info, err := c.cli.Info(ctx)
	if err != nil {
		return HostMetrics{}, err
	}

	m := HostMetrics{
		HostCPUPct:   -1,
		HostMemPct:   -1,
		DiskUsedPct:  -1,
		NCPU:         info.NCPU,
		HostMemTotal: uint64(info.MemTotal),
		Source:       "docker",
	}

	if !c.IsRemoteDaemon() && runtime.GOOS == "linux" {
		if cpu, ok := sampleLocalCPU(); ok {
			m.HostCPUPct = cpu
			m.Source = "local"
		}
		if used, total, pct, ok := sampleLocalMem(); ok {
			m.HostMemUsed = used
			m.HostMemTotal = total
			m.HostMemPct = pct
			m.Source = "local"
		}
		if pct, ok := sampleLocalDisk("/"); ok {
			m.DiskUsedPct = pct
		}
	}

	needDockerPressure := m.HostCPUPct < 0 || m.HostMemPct < 0
	if needDockerPressure {
		dockerCPU, dockerMem, dockerN, aggErr := c.aggregateRunningStats(ctx)
		if aggErr == nil {
			m.DockerCPUPct = dockerCPU
			m.DockerMem = dockerMem
			m.DockerN = dockerN
		}
	}

	// Remote (or local fallback): approximate host pressure from containers + Info.
	if m.HostCPUPct < 0 && m.NCPU > 0 && m.DockerN > 0 {
		m.HostCPUPct = m.DockerCPUPct / float64(m.NCPU)
		if m.HostCPUPct > 100 {
			m.HostCPUPct = 100
		}
	}
	if m.HostMemPct < 0 && m.HostMemTotal > 0 && m.DockerMem > 0 {
		m.HostMemUsed = m.DockerMem
		m.HostMemPct = float64(m.DockerMem) / float64(m.HostMemTotal) * 100
		if m.HostMemPct > 100 {
			m.HostMemPct = 100
		}
	}

	return m, nil
}

func (c *Client) aggregateRunningStats(ctx context.Context) (cpu float64, mem uint64, n int, err error) {
	list, err := c.cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("status", "running")),
	})
	if err != nil {
		return 0, 0, 0, err
	}
	if len(list) == 0 {
		return 0, 0, 0, nil
	}
	ids := make([]string, len(list))
	for i, ctr := range list {
		ids[i] = ctr.ID
	}
	cpu, mem, err = c.StatsAggregate(ctx, ids)
	return cpu, mem, len(ids), err
}

func sampleLocalCPU() (float64, bool) {
	idle, total, ok := readProcStat()
	if !ok {
		return 0, false
	}
	hostCPUMu.Lock()
	defer hostCPUMu.Unlock()
	cur := cpuSample{idle: idle, total: total}
	if !hostCPUInit {
		hostCPUPrev = cur
		hostCPUInit = true
		return 0, false
	}
	dIdle := cur.idle - hostCPUPrev.idle
	dTotal := cur.total - hostCPUPrev.total
	hostCPUPrev = cur
	if dTotal == 0 {
		return 0, true
	}
	busy := 1 - float64(dIdle)/float64(dTotal)
	if busy < 0 {
		busy = 0
	}
	if busy > 1 {
		busy = 1
	}
	return busy * 100, true
}

func readProcStat() (idle, total uint64, ok bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, false
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, false
	}
	var vals []uint64
	for _, field := range fields[1:] {
		v, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		vals = append(vals, v)
		total += v
	}
	idle = vals[3]
	if len(vals) > 4 {
		idle += vals[4]
	}
	return idle, total, true
}

func sampleLocalMem() (used, total uint64, pct float64, ok bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, false
	}
	defer f.Close()
	var memTotal, memAvail uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			memTotal = parseMeminfoKiB(line) * 1024
		case strings.HasPrefix(line, "MemAvailable:"):
			memAvail = parseMeminfoKiB(line) * 1024
		}
		if memTotal > 0 && memAvail > 0 {
			break
		}
	}
	if memTotal == 0 {
		return 0, 0, 0, false
	}
	if memAvail > memTotal {
		memAvail = memTotal
	}
	used = memTotal - memAvail
	pct = float64(used) / float64(memTotal) * 100
	return used, memTotal, pct, true
}

func parseMeminfoKiB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}

func sampleLocalDisk(path string) (float64, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, false
	}
	total := st.Blocks * uint64(st.Bsize)
	free := st.Bavail * uint64(st.Bsize)
	if total == 0 {
		return 0, false
	}
	used := total - free
	return float64(used) / float64(total) * 100, true
}

func demoHostMetrics() HostMetrics {
	t := float64(time.Now().UnixMilli()) / 1000
	cpu := 12 + 8*math.Sin(t*0.7) + float64(time.Now().UnixNano()%300)/100
	ram := 10.5 + 0.4*math.Sin(t*0.2)
	disk := 52.7
	dCPU := 2 + 40*math.Max(0, math.Sin(t*1.3)-0.55)
	dMem := uint64(1300+20*math.Sin(t*0.15)) * 1024 * 1024
	return HostMetrics{
		HostCPUPct:   clampPct(cpu),
		HostMemPct:   clampPct(ram),
		HostMemUsed:  uint64(ram / 100 * 16 * 1024 * 1024 * 1024),
		HostMemTotal: 16 * 1024 * 1024 * 1024,
		NCPU:         8,
		DiskUsedPct:  disk,
		DockerCPUPct: clampPct(dCPU),
		DockerMem:    dMem,
		DockerN:      2,
		Source:       "demo",
	}
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
