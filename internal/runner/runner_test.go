package runner

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCmdRunCapturesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("usa utilidades POSIX")
	}

	out, err := Cmd{}.Run(context.Background(), "", "echo", "hello", "world")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if out != "hello world" {
		t.Errorf("output = %q, want %q (debe venir recortado)", out, "hello world")
	}
}

func TestCmdRunUsesOutputAsErrorMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("usa utilidades POSIX")
	}

	_, err := Cmd{}.Run(context.Background(), "", "sh", "-c", "echo boom >&2; exit 1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want to contain stderr output 'boom'", err)
	}
}

func TestCmdRunRespectsTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("usa utilidades POSIX")
	}

	start := time.Now()
	_, err := Cmd{Timeout: 50 * time.Millisecond}.Run(context.Background(), "", "sleep", "5")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("timeout no respetado: tardó %v", elapsed)
	}
}

func TestCmdRunSetsWorkingDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("usa utilidades POSIX")
	}

	dir := t.TempDir()
	out, err := Cmd{}.Run(context.Background(), dir, "pwd")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	// En macOS /tmp es symlink a /private/tmp; basta con que termine en el sufijo.
	if !strings.HasSuffix(out, strings.TrimPrefix(dir, "/private")) && out != dir {
		t.Errorf("pwd = %q, want working dir %q", out, dir)
	}
}
