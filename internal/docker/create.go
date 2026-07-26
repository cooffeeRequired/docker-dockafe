package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/image"
)

type ComposeServiceSpec struct {
	Name        string
	Image       string
	Ports       []string // "8080:80"
	Env         []string // "KEY=val"
	Volumes     []string // "./data:/data" or "vol:/data"
	Restart     string   // unless-stopped, always, no
	Command     string
	DependsOn   []string
	Environment map[string]string
}

type ComposeProjectSpec struct {
	Name     string
	Dir      string
	Services []ComposeServiceSpec
}

func (c *Client) LocalImageTags(ctx context.Context) ([]string, error) {
	list, err := c.cli.ImageList(ctx, image.ListOptions{All: false})
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(list)*2)
	seen := map[string]struct{}{}
	for _, img := range list {
		for _, t := range img.RepoTags {
			if t == "" || t == "<none>:<none>" {
				continue
			}
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			tags = append(tags, t)
		}
	}
	return tags, nil
}

func (c *Client) PullImage(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty image reference")
	}
	rc, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return "", err
	}
	defer rc.Close()
	// Drain progress JSON stream
	_, _ = io.Copy(io.Discard, rc)
	return fmt.Sprintf("image pulled: %s", ref), nil
}

func (c *Client) RunTemporaryContainer(ctx context.Context, ref, name string, ports []string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty image reference")
	}
	if _, err := c.PullImage(ctx, ref); err != nil {
		// continue — image may already exist locally
		_ = err
	}

	args := []string{"run", "-d", "--rm"}
	if name != "" {
		args = append(args, "--name", name)
	}
	for _, p := range ports {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		args = append(args, "-p", p)
	}
	args = append(args, ref)
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	id := strings.TrimSpace(string(out))
	if len(id) > 12 {
		id = id[:12]
	}
	return fmt.Sprintf("temp container started (--rm): %s", id), nil
}

func (spec ComposeProjectSpec) RenderYAML() string {
	var b strings.Builder
	b.WriteString("name: " + yamlQuote(spec.Name) + "\n")
	b.WriteString("services:\n")
	for _, svc := range spec.Services {
		b.WriteString("  " + svc.Name + ":\n")
		b.WriteString("    image: " + yamlQuote(svc.Image) + "\n")
		if svc.Restart == "" {
			svc.Restart = "unless-stopped"
		}
		b.WriteString("    restart: " + svc.Restart + "\n")
		if len(svc.Ports) > 0 {
			b.WriteString("    ports:\n")
			for _, p := range svc.Ports {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				b.WriteString("      - " + yamlQuote(p) + "\n")
			}
		}
		envs := svc.Env
		if len(svc.Environment) > 0 {
			for k, v := range svc.Environment {
				envs = append(envs, k+"="+v)
			}
		}
		if len(envs) > 0 {
			b.WriteString("    environment:\n")
			for _, e := range envs {
				e = strings.TrimSpace(e)
				if e == "" {
					continue
				}
				b.WriteString("      - " + yamlQuote(e) + "\n")
			}
		}
		if len(svc.Volumes) > 0 {
			b.WriteString("    volumes:\n")
			for _, v := range svc.Volumes {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}
				b.WriteString("      - " + yamlQuote(v) + "\n")
			}
		}
		if len(svc.DependsOn) > 0 {
			b.WriteString("    depends_on:\n")
			for _, d := range svc.DependsOn {
				d = strings.TrimSpace(d)
				if d == "" {
					continue
				}
				b.WriteString("      - " + d + "\n")
			}
		}
		if strings.TrimSpace(svc.Command) != "" {
			b.WriteString("    command: " + yamlQuote(svc.Command) + "\n")
		}
	}
	return b.String()
}

func (c *Client) WriteComposeFile(spec ComposeProjectSpec, yamlBody string) (string, error) {
	dir := strings.TrimSpace(spec.Dir)
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "docker-projects", spec.Name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "compose.yaml")
	if yamlBody == "" {
		yamlBody = spec.RenderYAML()
	}
	if err := os.WriteFile(path, []byte(yamlBody), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (c *Client) ComposeUp(ctx context.Context, projectDir, projectName string) (string, error) {
	args := []string{"compose"}
	if projectName != "" {
		args = append(args, "-p", projectName)
	}
	args = append(args, "up", "-d")
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	msg := strings.TrimSpace(string(out))
	if err != nil {
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	if msg == "" {
		msg = "compose up -d OK"
	}
	return msg, nil
}

func (c *Client) ComposeConfigCheck(ctx context.Context, projectDir string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "config", "-q")
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func yamlQuote(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#{}[],&*?|-<>=!%@`\"'\n") || strings.Contains(s, " ") {
		s = strings.ReplaceAll(s, `"`, `\"`)
		return `"` + s + `"`
	}
	return s
}
