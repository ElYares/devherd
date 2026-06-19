package proxy

import (
	"strings"
	"testing"
)

func TestRenderExternalSiteWrapsWithMarkers(t *testing.T) {
	project := ExternalProject{
		Domain: "demo.localhost",
		Routes: []Route{{Matcher: "/*", Target: "demo-web:3000"}},
	}

	site := renderExternalSite(project)

	for _, want := range []string{
		"# devherd managed start demo.localhost",
		"http://demo.localhost {",
		"reverse_proxy demo-web:3000",
		"# devherd managed end demo.localhost",
	} {
		if !strings.Contains(site, want) {
			t.Errorf("rendered site missing %q:\n%s", want, site)
		}
	}
}

func TestStripManagedDomainsRemovesMarkedBlockKeepingOthers(t *testing.T) {
	managed := renderExternalSite(ExternalProject{
		Domain: "demo.localhost",
		Routes: []Route{{Matcher: "/*", Target: "demo-web:3000"}},
	})
	other := "http://keep.localhost {\n\thandle {\n\t\treverse_proxy keep:1\n\t}\n}"

	content := other + "\n\n" + managed

	stripped := stripManagedDomains(content, []string{"demo.localhost"})

	if strings.Contains(stripped, "demo.localhost") {
		t.Errorf("expected demo.localhost removed, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "keep.localhost") {
		t.Errorf("expected keep.localhost preserved, got:\n%s", stripped)
	}
}

func TestStripManagedDomainsMigratesLegacyUnmarkedBlock(t *testing.T) {
	// Bloque en formato viejo (sin marcadores) → debe eliminarse vía fallback de llaves.
	legacy := "http://legacy.localhost {\n\thandle {\n\t\treverse_proxy y:2\n\t}\n}"

	stripped := stripManagedDomains(legacy, []string{"legacy.localhost"})

	if strings.TrimSpace(stripped) != "" {
		t.Errorf("expected legacy block fully stripped, got:\n%s", stripped)
	}
}
