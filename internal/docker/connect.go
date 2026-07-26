package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
)

// Endpoint describes a Docker daemon connection target.
type Endpoint struct {
	Name    string // context name or "custom"
	Host    string // empty = default from env / unix socket
	Current bool
	Source  string // "context" | "env" | "custom"
}

// New creates a client from the environment (DOCKER_HOST / context).
func New() (*Client, error) {
	return NewWithHost("")
}

// NewWithHost connects to a specific Docker host URL.
// Empty host uses client.FromEnv (DOCKER_HOST / docker context).
func NewWithHost(host string) (*Client, error) {
	opts := []client.Opt{client.WithAPIVersionNegotiation()}
	label := "default"
	if strings.TrimSpace(host) != "" {
		opts = append(opts, client.WithHost(strings.TrimSpace(host)))
		label = strings.TrimSpace(host)
	} else {
		opts = append(opts, client.FromEnv)
		if h := strings.TrimSpace(os.Getenv("DOCKER_HOST")); h != "" {
			label = h
		} else {
			label = "unix:///var/run/docker.sock"
		}
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	c := &Client{cli: cli, hostLabel: label}
	if dh := cli.DaemonHost(); dh != "" {
		c.hostLabel = dh
	}
	return c, nil
}

// Host returns the daemon endpoint label shown in the UI.
func (c *Client) Host() string {
	if c == nil {
		return ""
	}
	if c.IsDemo() {
		return "demo"
	}
	if c.hostLabel != "" {
		return c.hostLabel
	}
	if c.cli != nil {
		return c.cli.DaemonHost()
	}
	return ""
}

// ListEndpoints returns docker contexts plus the current env host.
func ListEndpoints(ctx context.Context) ([]Endpoint, error) {
	out := []Endpoint{}
	seen := map[string]bool{}

	envHost := strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	if envHost != "" {
		out = append(out, Endpoint{Name: "env", Host: envHost, Source: "env"})
		seen[envHost] = true
	}

	cmd := exec.CommandContext(ctx, "docker", "context", "ls", "--format", "{{.Name}}\t{{.DockerEndpoint}}\t{{.Current}}")
	b, err := cmd.Output()
	if err == nil {
		sc := bufio.NewScanner(strings.NewReader(string(b)))
		for sc.Scan() {
			line := sc.Text()
			parts := strings.Split(line, "\t")
			if len(parts) < 2 {
				continue
			}
			name := strings.TrimSpace(parts[0])
			host := strings.TrimSpace(parts[1])
			current := len(parts) > 2 && (parts[2] == "true" || parts[2] == "True" || parts[2] == "*")
			if host == "" {
				continue
			}
			if seen[host] && name != "default" {
				// still add named context
			}
			seen[host] = true
			out = append(out, Endpoint{
				Name:    name,
				Host:    host,
				Current: current,
				Source:  "context",
			})
		}
	}

	// Always offer local unix socket as explicit choice.
	local := "unix:///var/run/docker.sock"
	if !seen[local] {
		out = append([]Endpoint{{Name: "local", Host: local, Source: "builtin"}}, out...)
	}

	if len(out) == 0 {
		out = append(out, Endpoint{Name: "local", Host: local, Source: "builtin"})
	}
	return out, nil
}

// ContextEndpoint resolves a docker context name to its Docker endpoint URL.
func ContextEndpoint(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("empty context name")
	}
	cmd := exec.CommandContext(ctx, "docker", "context", "inspect", name, "--format", "{{json .Endpoints.docker.Host}}")
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker context inspect: %w", err)
	}
	var host string
	if err := json.Unmarshal(b, &host); err != nil {
		host = strings.Trim(strings.TrimSpace(string(b)), `"`)
	}
	if host == "" {
		return "", fmt.Errorf("context %q has empty host", name)
	}
	return host, nil
}

// DefaultDownloadDir is ~/Downloads/dockafe or /tmp/dockafe.
func DefaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "dockafe")
	}
	return filepath.Join(home, "Downloads", "dockafe")
}
