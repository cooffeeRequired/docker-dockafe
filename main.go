package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cooffeeRequired/dockafe/internal/docker"
	"github.com/cooffeeRequired/dockafe/internal/ui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help", "help":
			fmt.Println(`Dockafé — interactive Docker TUI

Usage:
  dockafe

Requires running Docker daemon (DOCKER_HOST / docker.sock).
Press ? inside the app for keybindings.`)
			return
		case "-v", "--version", "version":
			fmt.Printf("%s %s\n", ui.AppName, ui.AppVersion)
			return
		}
	}

	client, err := docker.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "docker daemon unavailable: %v\n", err)
		fmt.Fprintln(os.Stderr, "Start Docker and verify DOCKER_HOST / socket permissions.")
		os.Exit(1)
	}

	p := tea.NewProgram(
		ui.New(client),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
}
