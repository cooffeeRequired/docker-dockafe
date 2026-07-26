package ui

const statsHistCap = 60

type statsSample struct {
	cpu float64
	mem uint64
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

func (m *Model) recordStatsFromData(msg dataMsg) {
	m.ensureStatsHist()
	for _, c := range msg.containers {
		if !c.Running {
			continue
		}
		m.seriesFor(c.ID).push(c.CPUVal, c.MemBytes)
	}
	for _, g := range msg.groups {
		m.seriesFor(composeHistKey(g.Name)).push(g.CPUVal, g.MemBytes)
	}
}
