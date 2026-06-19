package dns

import (
	"strings"
	"testing"
)

func TestValidateDomainsNormalizesAndDedupes(t *testing.T) {
	got, err := validateDomains([]string{"App.Test", " app.test ", "api.localhost", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"app.test", "api.localhost"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestValidateDomainsRejectsInjection(t *testing.T) {
	bad := []string{
		"evil.test 10.0.0.1 other",  // espacios → entrada extra
		"line.test\n127.0.0.1 evil", // salto de línea
		"semi;colon.test",           // metacaracter
		"under_score.test",          // guion bajo no permitido
		"-leading.test",             // guion al inicio
	}
	for _, d := range bad {
		if _, err := validateDomains([]string{d}); err == nil {
			t.Errorf("expected error for malicious domain %q", d)
		}
	}
}

func TestValidateDomainsAcceptsValid(t *testing.T) {
	valid := []string{"my-app.test", "foo.localhost", "a.b.c.test", "app123.test"}
	for _, d := range valid {
		if _, err := validateDomains([]string{d}); err != nil {
			t.Errorf("expected %q to be valid, got %v", d, err)
		}
	}
}
