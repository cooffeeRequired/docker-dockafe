package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/mattn/go-runewidth"

	"github.com/cooffeeRequired/dockafe/internal/docker"
)

func (m Model) sortKeysForTab() []SortKey {
	switch m.tab {
	case TabCompose:
		return []SortKey{SortName, SortState, SortCPU, SortMem}
	case TabContainers:
		return []SortKey{SortName, SortProject, SortState, SortCPU, SortMem, SortCreated}
	case TabImages:
		return []SortKey{SortName, SortSize, SortCreated}
	case TabVolumes:
		return []SortKey{SortName, SortState, SortCreated}
	default:
		return []SortKey{SortName}
	}
}

func (m Model) sortLabel() string {
	dir := "↑"
	if !m.sortAsc {
		dir = "↓"
	}
	name := map[SortKey]string{
		SortName:    "name",
		SortState:   "state",
		SortCPU:     "cpu",
		SortMem:     "mem",
		SortSize:    "size",
		SortCreated: "created",
		SortProject: "project",
	}[m.sortKey]
	if m.tab == TabVolumes && m.sortKey == SortState {
		name = "in-use"
	}
	return name + dir
}

func (m *Model) cycleSort() {
	keys := m.sortKeysForTab()
	if len(keys) == 0 {
		return
	}
	idx := 0
	for i, k := range keys {
		if k == m.sortKey {
			idx = i
			break
		}
	}
	m.sortKey = keys[(idx+1)%len(keys)]
}

func matchesFilter(q string, fields ...string) bool {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return true
	}
	parts := strings.Fields(q)
	for _, part := range parts {
		ok := false
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f), part) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func (m Model) filteredGroups() []docker.ComposeGroup {
	q := m.filter.Value()
	out := make([]docker.ComposeGroup, 0, len(m.groups))
	for _, g := range m.groups {
		if !matchesFilter(q, g.Name, g.Ports, g.CPU, g.Mem) {
			continue
		}
		if m.runningOnly && g.Running == 0 {
			continue
		}
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		var less bool
		switch m.sortKey {
		case SortState:
			less = a.Running < b.Running
		case SortCPU:
			less = a.CPUVal < b.CPUVal
		case SortMem:
			less = a.MemBytes < b.MemBytes
		default:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		if m.sortAsc {
			return less
		}
		return !less
	})
	return out
}

func (m Model) filteredContainers() []docker.ContainerInfo {
	q := m.filter.Value()
	src := m.containers
	out := make([]docker.ContainerInfo, 0, len(src))
	for _, c := range src {
		if m.selectedGroup != "" {
			key := c.Project
			if key == "" {
				key = "(standalone)"
			}
			if key != m.selectedGroup {
				continue
			}
		}
		if m.runningOnly && !c.Running {
			continue
		}
		if !matchesFilter(q, c.Name, c.Image, c.Project, c.Service, c.State, c.Status, c.Health, c.Ports, short(c.ID)) {
			continue
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		var less bool
		switch m.sortKey {
		case SortProject:
			less = strings.ToLower(a.Project) < strings.ToLower(b.Project)
		case SortState:
			less = a.State < b.State
		case SortCPU:
			less = a.CPUVal < b.CPUVal
		case SortMem:
			less = a.MemBytes < b.MemBytes
		case SortCreated:
			less = a.Created.Before(b.Created)
		default:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		if m.sortAsc {
			return less
		}
		return !less
	})
	return out
}

func (m Model) filteredImages() []docker.ImageInfo {
	q := m.filter.Value()
	out := make([]docker.ImageInfo, 0, len(m.images))
	for _, img := range m.images {
		if !matchesFilter(q, img.ID, img.Tags, img.Size) {
			continue
		}
		out = append(out, img)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		var less bool
		switch m.sortKey {
		case SortSize:
			less = a.SizeBytes < b.SizeBytes
		case SortCreated:
			less = a.Created.Before(b.Created)
		default:
			less = strings.ToLower(a.Tags) < strings.ToLower(b.Tags)
		}
		if m.sortAsc {
			return less
		}
		return !less
	})
	return out
}

func (m Model) filteredVolumes() []docker.VolumeInfo {
	q := m.filter.Value()
	out := make([]docker.VolumeInfo, 0, len(m.volumes))
	for _, v := range m.volumes {
		if m.runningOnly && !v.InUse {
			continue
		}
		if !matchesFilter(q, v.Name, v.Driver, v.Mountpoint, v.Scope, v.UsedBy) {
			continue
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		var less bool
		switch m.sortKey {
		case SortCreated:
			less = a.Created.Before(b.Created)
		case SortState:
			if a.InUse != b.InUse {
				less = a.InUse
			} else {
				less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
			}
		default:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		if m.sortAsc {
			return less
		}
		return !less
	})
	return out
}

func (m Model) filteredNetworks() []docker.NetworkInfo {
	q := m.filter.Value()
	out := make([]docker.NetworkInfo, 0, len(m.networks))
	for _, n := range m.networks {
		if !matchesFilter(q, n.Name, n.Driver, n.Scope, n.Subnet, n.ID) {
			continue
		}
		out = append(out, n)
	}
	sort.SliceStable(out, func(i, j int) bool {
		less := strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		if m.sortAsc {
			return less
		}
		return !less
	})
	return out
}

func (m *Model) applyRows() {
	cursor := m.table.Cursor()
	m.table.SetRows(nil)
	width := m.width - 4
	if width < 60 {
		width = 60
	}
	m.table.SetColumns(defaultColumns(m.tab, width))

	var rows []table.Row
	switch m.tab {
	case TabSettings:
		// Settings uses a dedicated view, not the resource table.
	case TabCompose:
		for _, g := range m.filteredGroups() {
			rows = append(rows, table.Row{
				g.Name,
				fmt.Sprintf("%d/%d", g.Running, g.Total),
				g.CPU,
				g.Mem,
				truncate(g.Ports, 48),
			})
		}
	case TabContainers:
		for _, c := range m.filteredContainers() {
			proj := c.Project
			if proj == "" {
				proj = "-"
			}
			rows = append(rows, table.Row{
				c.Name,
				proj,
				docker.StateLabel(c.State, c.Health, c.OOMKilled),
				c.CPU,
				c.Mem,
				truncate(c.Ports, 36),
				short(c.ID),
			})
		}
	case TabImages:
		for _, img := range m.filteredImages() {
			rows = append(rows, table.Row{
				img.ID,
				truncate(img.Tags, 50),
				img.Size,
				img.Created.Format("2006-01-02 15:04"),
			})
		}
	case TabVolumes:
		for _, v := range m.filteredVolumes() {
			used := "-"
			if v.InUse {
				used = fmt.Sprintf("%d · %s", v.RefCount, truncate(v.UsedBy, 28))
			}
			rows = append(rows, table.Row{
				truncate(v.Name, 28),
				v.Driver,
				used,
				truncate(v.Mountpoint, 40),
			})
		}
	case TabNetworks:
		for _, n := range m.filteredNetworks() {
			rows = append(rows, table.Row{
				n.Name,
				n.Driver,
				n.Scope,
				emptyDash(n.Subnet),
				n.ID,
			})
		}
	}

	m.table.SetRows(rows)
	if len(rows) == 0 {
		m.table.SetCursor(0)
		return
	}
	if cursor >= len(rows) {
		cursor = len(rows) - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	m.table.SetCursor(cursor)
}

func defaultColumns(tab Tab, width int) []table.Column {
	if width < 60 {
		width = 60
	}
	budget := width - 2
	var cols []table.Column
	switch tab {
	case TabCompose:
		cols = []table.Column{
			{Title: "PROJECT", Width: max(16, width/5)},
			{Title: "UP", Width: 7},
			{Title: "CPU", Width: 8},
			{Title: "MEM", Width: 18},
			{Title: "PORTS / EXPORT", Width: max(20, width/3)},
		}
	case TabContainers:
		cols = []table.Column{
			{Title: "NAME", Width: max(14, width/6)},
			{Title: "PROJECT", Width: 14},
			{Title: "STATE", Width: 11},
			{Title: "CPU", Width: 7},
			{Title: "MEM", Width: 16},
			{Title: "PORTS", Width: max(16, width/4)},
			{Title: "ID", Width: 12},
		}
	case TabImages:
		cols = []table.Column{
			{Title: "ID", Width: 12},
			{Title: "TAGS", Width: max(24, width/2)},
			{Title: "SIZE", Width: 10},
			{Title: "CREATED", Width: 16},
		}
	case TabVolumes:
		cols = []table.Column{
			{Title: "NAME", Width: max(16, width/4)},
			{Title: "DRIVER", Width: 8},
			{Title: "IN USE", Width: max(18, width/4)},
			{Title: "MOUNTPOINT", Width: max(20, width/3)},
		}
	case TabSettings:
		cols = []table.Column{
			{Title: "SETTING", Width: max(20, width/2)},
			{Title: "VALUE", Width: max(20, width/2)},
		}
	default:
		cols = []table.Column{
			{Title: "NAME", Width: max(14, width/5)},
			{Title: "DRIVER", Width: 10},
			{Title: "SCOPE", Width: 8},
			{Title: "SUBNET", Width: max(16, width/3)},
			{Title: "ID", Width: 12},
		}
	}
	return fitColumns(cols, budget)
}

func fitColumns(cols []table.Column, budget int) []table.Column {
	sum := 0
	for _, c := range cols {
		sum += c.Width
	}
	if sum <= budget || sum == 0 {
		return cols
	}
	out := make([]table.Column, len(cols))
	copy(out, cols)
	for sum > budget {
		shrunk := false
		for i := range out {
			if out[i].Width > 6 {
				out[i].Width--
				sum--
				shrunk = true
				if sum <= budget {
					break
				}
			}
		}
		if !shrunk {
			break
		}
	}
	return out
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return runewidth.Truncate(s, n, "…")
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
