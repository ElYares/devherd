package proxy

import (
	"strings"
	"testing"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/database"
)

func TestRenderVueFlaskSite(t *testing.T) {
	renderer := NewRenderer(config.Paths{}, config.Default())

	content, domains, err := renderer.Render([]database.ProjectRecord{
		{
			Name:      "hello-vue-flask-docker",
			Framework: "vue+flask",
			Domain:    "mi-demo.test",
		},
	})
	if err != nil {
		t.Fatalf("render proxy config: %v", err)
	}

	if len(domains) != 1 || domains[0] != "mi-demo.test" {
		t.Fatalf("unexpected domains: %#v", domains)
	}

	expectedFragments := []string{
		"mi-demo.test {",
		"path /api/*",
		"reverse_proxy 127.0.0.1:8000",
		"reverse_proxy 127.0.0.1:5173",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(content, fragment) {
			t.Fatalf("expected fragment %q in rendered caddyfile", fragment)
		}
	}
}

