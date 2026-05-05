package doctor

import (
	"strings"
	"testing"
)

func TestProcNetContainsListeningPort(t *testing.T) {
	payload := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000   100        0 1 1 0000000000000000 100 0 0 10 0
`

	if !procNetContainsListeningPort(payload, 80) {
		t.Fatal("expected port 80 to be detected as listening")
	}

	if procNetContainsListeningPort(payload, 443) {
		t.Fatal("did not expect port 443 to be detected as listening")
	}
}

func TestWindowsNetstatContainsListeningPort(t *testing.T) {
	output := `
  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:80             0.0.0.0:0              LISTENING       1000
  TCP    [::]:443               [::]:0                 LISTENING       2000
`

	if !windowsNetstatContainsListeningPort(output, 80) {
		t.Fatal("expected port 80 to be detected as listening")
	}

	if !windowsNetstatContainsListeningPort(output, 443) {
		t.Fatal("expected port 443 to be detected as listening")
	}

	if windowsNetstatContainsListeningPort(output, 3000) {
		t.Fatal("did not expect port 3000 to be detected as listening")
	}
}

func TestParseDockerEngineInfo(t *testing.T) {
	info := parseDockerEngineInfo("linux\tDocker Desktop 4.41.2\tmoby")

	if info.OSType != "linux" {
		t.Fatalf("expected OSType linux, got %q", info.OSType)
	}

	if info.OperatingSystem != "Docker Desktop 4.41.2" {
		t.Fatalf("unexpected operating system: %q", info.OperatingSystem)
	}

	if info.Name != "moby" {
		t.Fatalf("unexpected engine name: %q", info.Name)
	}
}

func TestParseDockerNetworkInfo(t *testing.T) {
	info, err := parseDockerNetworkInfo("bridge\tlocal\tfalse")
	if err != nil {
		t.Fatalf("parseDockerNetworkInfo returned error: %v", err)
	}

	if info.Driver != "bridge" || info.Scope != "local" || info.Internal {
		t.Fatalf("unexpected network info: %#v", info)
	}
}

func TestCheckManagedSuffixForOS(t *testing.T) {
	okCheck := checkManagedSuffixForOS("darwin", "localhost")
	if okCheck.Status != StatusOK {
		t.Fatalf("expected localhost suffix to be ok on darwin, got %s", okCheck.Status)
	}

	warnCheck := checkManagedSuffixForOS("windows", "test")
	if warnCheck.Status != StatusWarn {
		t.Fatalf("expected non-localhost suffix to warn on windows, got %s", warnCheck.Status)
	}

	if !strings.Contains(warnCheck.Message, ".test") {
		t.Fatalf("expected warning message to mention suffix, got %q", warnCheck.Message)
	}
}
