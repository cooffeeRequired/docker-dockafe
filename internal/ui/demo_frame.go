package ui

import (
	"context"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/cooffeeRequired/dockafe/internal/docker"
)

func init() {
	// Force colors when rendering off a TTY (screenshot generation).
	lipgloss.SetColorProfile(termenv.ANSI256)
}

// DemoFrameKind selects which canned screen to render.
type DemoFrameKind string

const (
	DemoFrameSplash      DemoFrameKind = "splash"
	DemoFrameCompose     DemoFrameKind = "compose"
	DemoFrameVolumes     DemoFrameKind = "volumes"
	DemoFrameVolumeFiles DemoFrameKind = "volume-files"
)

// RenderDemoFrame returns a colored terminal frame with sample data (no Docker).
func RenderDemoFrame(kind DemoFrameKind, width, height int) string {
	client := docker.NewDemo()
	m := New(client)
	m.width = width
	m.height = height
	m.ready = true
	m.loading = false
	m.lastSync = time.Date(2026, 7, 26, 15, 24, 55, 0, time.Local)
	m.sysInfo = "Docker 29.0.0 · Demo Linux/x86_64 · containers 12 (running 8) · images 6 · CPUs 8 · Mem 7.8GiB"
	m.status = "sync 15:24:55 · sort name↑"

	ctx := context.Background()
	groups, _ := client.ListComposeGroups(ctx, true)
	containers, _ := client.ListContainers(ctx, true)
	images, _ := client.ListImages(ctx)
	volumes, _ := client.ListVolumes(ctx)
	networks, _ := client.ListNetworks(ctx)
	m.groups = groups
	m.containers = containers
	m.images = images
	m.volumes = volumes
	m.networks = networks

	switch kind {
	case DemoFrameSplash:
		m.mode = ModeSplash
		m.splashMinDone = false
		m.splashDataReady = false
	case DemoFrameCompose:
		m.mode = ModeList
		m.tab = TabCompose
		m.relayout()
		m.applyRows()
	case DemoFrameVolumes:
		m.mode = ModeList
		m.tab = TabVolumes
		m.relayout()
		m.applyRows()
	case DemoFrameVolumeFiles:
		m.mode = ModeVolumeTree
		m.volName = "shop-api_pgdata"
		m.volAccessMode = "via docker"
		m.volRoot = &volNode{
			entry:    docker.VolumeEntry{Name: "shop-api_pgdata", Path: "", IsDir: true},
			expanded: true,
			loaded:   true,
		}
		for _, e := range docker.DemoVolumeEntries() {
			child := &volNode{entry: e, depth: 0}
			if e.IsDir {
				child.expanded = false
			}
			m.volRoot.children = append(m.volRoot.children, child)
		}
		m.volCursor = 0
		m.volPreviewPath = "postgresql.conf"
		m.volPreview = highlightSource("postgresql.conf", docker.DemoVolumeFile("postgresql.conf"))
		m.volFileFocus = false
		m.status = "preview · postgresql.conf · via docker"
		m.relayout()
	default:
		m.mode = ModeList
		m.tab = TabCompose
		m.relayout()
		m.applyRows()
	}
	return m.View()
}
