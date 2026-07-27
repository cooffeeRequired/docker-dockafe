package ui

import "github.com/cooffeeRequired/dockafe/internal/docker"

const statsHistCap = 150

type statsSample struct {
	cpu float64
	mem uint64
}

// metricSeries is a capped float history used by dashboard panels.
type metricSeries struct {
	values []float64
	cap    int
}

func newMetricSeries(cap int) *metricSeries {
	if cap < 1 {
		cap = statsHistCap
	}
	return &metricSeries{cap: cap}
}

func (s *metricSeries) push(v float64) {
	if s == nil {
		return
	}
	if s.cap < 1 {
		s.cap = statsHistCap
	}
	s.values = append(s.values, v)
	if len(s.values) > s.cap {
		s.values = s.values[len(s.values)-s.cap:]
	}
}

func (s *metricSeries) len() int {
	if s == nil {
		return 0
	}
	return len(s.values)
}

func (s *metricSeries) last() (float64, bool) {
	if s == nil || len(s.values) == 0 {
		return 0, false
	}
	return s.values[len(s.values)-1], true
}

func (s *metricSeries) snapshot() []float64 {
	if s == nil || len(s.values) == 0 {
		return nil
	}
	out := make([]float64, len(s.values))
	copy(out, s.values)
	return out
}

type statsSeries struct {
	samples []statsSample
	cap     int
}

func newStatsSeries(cap int) *statsSeries {
	if cap < 1 {
		cap = statsHistCap
	}
	return &statsSeries{cap: cap}
}

func (s *statsSeries) push(cpu float64, mem uint64) {
	if s == nil {
		return
	}
	if s.cap < 1 {
		s.cap = statsHistCap
	}
	s.samples = append(s.samples, statsSample{cpu: cpu, mem: mem})
	if len(s.samples) > s.cap {
		s.samples = s.samples[len(s.samples)-s.cap:]
	}
}

func (s *statsSeries) len() int {
	if s == nil {
		return 0
	}
	return len(s.samples)
}

func (s *statsSeries) cpuValues() []float64 {
	if s == nil || len(s.samples) == 0 {
		return nil
	}
	out := make([]float64, len(s.samples))
	for i, sample := range s.samples {
		out[i] = sample.cpu
	}
	return out
}

func (s *statsSeries) memValues() []float64 {
	if s == nil || len(s.samples) == 0 {
		return nil
	}
	out := make([]float64, len(s.samples))
	for i, sample := range s.samples {
		out[i] = float64(sample.mem)
	}
	return out
}

func (s *statsSeries) last() (statsSample, bool) {
	if s == nil || len(s.samples) == 0 {
		return statsSample{}, false
	}
	return s.samples[len(s.samples)-1], true
}

func composeHistKey(name string) string {
	return "compose:" + name
}

func (m *Model) ensureStatsHist() {
	if m.statsHist == nil {
		m.statsHist = map[string]*statsSeries{}
	}
}

func (m *Model) seriesFor(key string) *statsSeries {
	m.ensureStatsHist()
	s, ok := m.statsHist[key]
	if !ok {
		s = newStatsSeries(statsHistCap)
		m.statsHist[key] = s
	}
	return s
}

func containerHasStats(c docker.ContainerInfo) bool {
	if c.MemBytes > 0 {
		return true
	}
	return c.CPU != "" && c.CPU != "-"
}

// mergePreservedStats keeps previous CPU/MEM when the new inventory row has none yet.
func mergePreservedStats(prev, next []docker.ContainerInfo) []docker.ContainerInfo {
	if len(prev) == 0 || len(next) == 0 {
		return next
	}
	byID := make(map[string]docker.ContainerInfo, len(prev))
	for _, c := range prev {
		byID[c.ID] = c
	}
	out := make([]docker.ContainerInfo, len(next))
	for i, c := range next {
		if !containerHasStats(c) {
			if old, ok := byID[c.ID]; ok && containerHasStats(old) {
				c.CPU = old.CPU
				c.Mem = old.Mem
				c.CPUVal = old.CPUVal
				c.MemBytes = old.MemBytes
			}
		}
		out[i] = c
	}
	return out
}

// applyStatsByID copies CPU/MEM from sampled rows onto the current inventory.
func applyStatsByID(current, sampled []docker.ContainerInfo) []docker.ContainerInfo {
	if len(current) == 0 {
		return sampled
	}
	byID := make(map[string]docker.ContainerInfo, len(sampled))
	for _, c := range sampled {
		if containerHasStats(c) {
			byID[c.ID] = c
		}
	}
	out := make([]docker.ContainerInfo, len(current))
	for i, c := range current {
		if s, ok := byID[c.ID]; ok {
			c.CPU = s.CPU
			c.Mem = s.Mem
			c.CPUVal = s.CPUVal
			c.MemBytes = s.MemBytes
		}
		out[i] = c
	}
	return out
}

func (m *Model) recordStatsFromData(msg dataMsg) {
	m.ensureStatsHist()
	for _, c := range msg.containers {
		if !c.Running || !containerHasStats(c) {
			continue
		}
		m.seriesFor(c.ID).push(c.CPUVal, c.MemBytes)
	}
	for _, g := range msg.groups {
		if g.CPU == "" || g.CPU == "-" {
			continue
		}
		m.seriesFor(composeHistKey(g.Name)).push(g.CPUVal, g.MemBytes)
	}
}
