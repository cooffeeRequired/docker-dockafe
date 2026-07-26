package docker_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/coffee/docker-tui/internal/docker"
)

func TestDockerIntegration(t *testing.T) {
	if os.Getenv("DOCKER_INTEGRATION") != "1" {
		t.Skip("set DOCKER_INTEGRATION=1 to run")
	}
	client, err := docker.New()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListVolumes(ctx); err != nil {
		t.Fatal(err)
	}
}
