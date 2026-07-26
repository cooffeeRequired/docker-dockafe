package docker

import (
	"strings"
	"testing"
)

func TestRenderYAMLQuotesKeysAndDepends(t *testing.T) {
	yaml := ComposeProjectSpec{
		Name: "demo",
		Services: []ComposeServiceSpec{
			{
				Name:      "web:evil",
				Image:     "nginx:alpine",
				Restart:   "totally-invalid",
				DependsOn: []string{"db:1", "cache"},
			},
		},
	}.RenderYAML()

	if !strings.Contains(yaml, `"web:evil":`) {
		t.Fatalf("service key not quoted:\n%s", yaml)
	}
	if !strings.Contains(yaml, "restart: unless-stopped") {
		t.Fatalf("restart not normalized:\n%s", yaml)
	}
	if !strings.Contains(yaml, `- "db:1"`) {
		t.Fatalf("depends_on not quoted:\n%s", yaml)
	}
	if strings.Contains(yaml, "totally-invalid") {
		t.Fatalf("invalid restart leaked:\n%s", yaml)
	}
}

func TestYamlKeySafe(t *testing.T) {
	if got := yamlKey("web"); got != "web" {
		t.Fatalf("got %q", got)
	}
	if got := yamlKey("web:1"); got != `"web:1"` {
		t.Fatalf("got %q", got)
	}
}
