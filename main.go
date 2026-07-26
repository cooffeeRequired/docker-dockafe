package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cooffeeRequired/dockafe/internal/docker"
	"github.com/cooffeeRequired/dockafe/internal/ui"
)

func main() {
	demo := false
	host := ""
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help" || a == "help":
			fmt.Println(`Dockafé — interactive Docker TUI

Usage:
  dockafe
  dockafe --demo
  dockafe --host ssh://user@server
  dockafe --host tcp://192.168.1.10:2375

  --demo         sample data only (no Docker socket)
  --host URL     connect to a remote/local Docker daemon

Requires a Docker daemon (DOCKER_HOST / docker.sock / --host), unless --demo.
Inside the app: H = switch host, ? = keybindings.`)
			return
		case a == "-v" || a == "--version" || a == "version":
			fmt.Printf("%s %s\n", ui.AppName, ui.AppVersion)
			return
		case a == "--demo" || a == "-demo" || a == "demo":
			demo = true
		case a == "--host" || a == "-H":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "missing value for --host")
				os.Exit(2)
			}
			i++
			host = strings.TrimSpace(args[i])
		case strings.HasPrefix(a, "--host="):
			host = strings.TrimSpace(strings.TrimPrefix(a, "--host="))
		default:
			fmt.Fprintf(os.Stderr, "unknown argument: %s\n", a)
			os.Exit(2)
		}
	}

	var client *docker.Client
	var err error
	if demo {
		client = docker.NewDemo()
	} else {
		client, err = docker.NewWithHost(host)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docker client: %v\n", err)
			os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Ping(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "docker daemon unavailable (%s): %v\n", client.Host(), err)
			fmt.Fprintln(os.Stderr, "Start Docker, set DOCKER_HOST, or pass --host. Or: dockafe --demo")
			os.Exit(1)
		}
	}
	defer client.Close()

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
