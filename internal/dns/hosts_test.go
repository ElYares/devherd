package dns

import (
	"strings"
	"testing"
)

func TestMergeManagedBlock(t *testing.T) {
	original := "127.0.0.1 localhost\n::1 localhost ip6-localhost\n"
	updated := mergeManagedBlock(original, []string{"mi-demo.test", "api-lab.local"})

	if !strings.Contains(updated, blockStart) {
		t.Fatal("expected managed block start marker")
	}

	if !strings.Contains(updated, "127.0.0.1 mi-demo.test api-lab.local") {
		t.Fatal("expected managed loopback mapping")
	}

	if !strings.Contains(updated, "::1 localhost ip6-localhost") {
		t.Fatal("expected original hosts content to be preserved")
	}
}

