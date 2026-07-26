package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cooffeeRequired/dockafe/internal/ui"
)

func main() {
	outDir := "docs/assets"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}

	frames := []struct {
		kind ui.DemoFrameKind
		file string
	}{
		{ui.DemoFrameSplash, "screenshot-splash.ansi"},
		{ui.DemoFrameCompose, "screenshot-compose.ansi"},
		{ui.DemoFrameVolumes, "screenshot-volumes.ansi"},
		{ui.DemoFrameVolumeFiles, "screenshot-volume-files.ansi"},
	}

	for _, f := range frames {
		frame := ui.RenderDemoFrame(f.kind, 110, 32)
		path := filepath.Join(outDir, f.file)
		if err := os.WriteFile(path, []byte(frame+"\n"), 0o644); err != nil {
			fatal(err)
		}
		fmt.Println("wrote", path)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
