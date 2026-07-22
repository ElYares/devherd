package observe

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderUnitWithoutAddr(t *testing.T) {
	unit, err := RenderUnit("/home/dev/.local/bin/devherd", "")
	if err != nil {
		t.Fatalf("RenderUnit returned error: %v", err)
	}

	if !strings.Contains(unit, "ExecStart=/home/dev/.local/bin/devherd observe start\n") {
		t.Errorf("unit does not start the collector with defaults:\n%s", unit)
	}
	if strings.Contains(unit, "--addr") {
		t.Errorf("empty addr must not render the flag:\n%s", unit)
	}
	if !strings.Contains(unit, "Restart=on-failure") {
		t.Errorf("unit must restart on failure:\n%s", unit)
	}
}

func TestRenderUnitWithAddr(t *testing.T) {
	unit, err := RenderUnit("/usr/bin/devherd", "172.18.0.1:9777")
	if err != nil {
		t.Fatalf("RenderUnit returned error: %v", err)
	}

	if !strings.Contains(unit, "ExecStart=/usr/bin/devherd observe start --addr 172.18.0.1:9777") {
		t.Errorf("unit does not carry the address:\n%s", unit)
	}
}

func TestRenderUnitRequiresBinary(t *testing.T) {
	if _, err := RenderUnit("   ", ""); err == nil {
		t.Fatal("expected an error without a binary path")
	}
}

func TestUnitPathFollowsXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, err := UnitPath()
	if err != nil {
		t.Fatalf("UnitPath returned error: %v", err)
	}

	want := filepath.Join(dir, "systemd", "user", UnitName)
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}
