package observe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProbeFromContainerUsesFirstLocalImage(t *testing.T) {
	// busybox ausente, alpine presente: la sonda tiene que caer en alpine y
	// lanzar el wget con --network de la red compartida.
	r := &fakeRunner{
		outputs: []string{"", "sha256:alpine", "ok"},
		errs:    []error{errors.New("no such image"), nil, nil},
	}

	result := ProbeFromContainer(context.Background(), r, "infra_web", "172.18.0.1:9777")
	if !result.Reachable {
		t.Fatalf("expected the collector to be reachable, got %+v", result)
	}
	if result.Image != "alpine" {
		t.Errorf("image = %q, want alpine", result.Image)
	}

	probe := strings.Join(r.calls[2], " ")
	for _, want := range []string{"run --rm", "--network infra_web", "--entrypoint wget", "http://172.18.0.1:9777/health"} {
		if !strings.Contains(probe, want) {
			t.Errorf("probe call %q is missing %q", probe, want)
		}
	}
}

func TestProbeFromContainerSkipsWithoutLocalImage(t *testing.T) {
	r := &fakeRunner{errs: []error{
		errors.New("no such image"),
		errors.New("no such image"),
		errors.New("no such image"),
	}}

	result := ProbeFromContainer(context.Background(), r, "infra_web", "172.18.0.1:9777")
	if !result.Skipped {
		t.Fatalf("expected the probe to be skipped, got %+v", result)
	}
	if result.Reachable {
		t.Error("a skipped probe must not report the collector as reachable")
	}
	if len(r.calls) != len(probeImages) {
		t.Errorf("expected one inspect per candidate image, got %d calls", len(r.calls))
	}
}

func TestProbeFromContainerReportsFailure(t *testing.T) {
	r := &fakeRunner{
		outputs: []string{"sha256:busybox", ""},
		errs:    []error{nil, errors.New("wget: download timed out")},
	}

	result := ProbeFromContainer(context.Background(), r, "infra_web", "172.18.0.1:9777")
	if result.Reachable || result.Skipped {
		t.Fatalf("expected a plain failure, got %+v", result)
	}
	if !strings.Contains(result.Reason, "timed out") {
		t.Errorf("reason = %q, want the docker error", result.Reason)
	}
}

func TestProbeFromContainerSkipsWithoutNetwork(t *testing.T) {
	r := &fakeRunner{}

	result := ProbeFromContainer(context.Background(), r, "", "172.18.0.1:9777")
	if !result.Skipped {
		t.Fatalf("expected the probe to be skipped, got %+v", result)
	}
	if len(r.calls) != 0 {
		t.Errorf("expected no docker calls, got %v", r.calls)
	}
}

func TestFirewallHintSuggestsUFWRule(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "ufw.conf")
	if err := os.WriteFile(conf, []byte("# comment\nENABLED=yes\n"), 0o644); err != nil {
		t.Fatalf("write ufw.conf: %v", err)
	}

	original := ufwConfPath
	ufwConfPath = conf
	t.Cleanup(func() { ufwConfPath = original })

	info := NetworkInfo{Name: "infra_web", Gateway: "172.18.0.1", Subnet: "172.18.0.0/16"}
	hint := FirewallHint(info, "172.18.0.1:9777")
	if !strings.Contains(hint, "ufw is enabled") {
		t.Errorf("hint = %q, want the enabled-ufw wording", hint)
	}
	if !strings.Contains(hint, "sudo ufw allow from 172.18.0.0/16 to 172.18.0.1 port 9777 proto tcp") {
		t.Errorf("hint = %q, want the concrete rule", hint)
	}
}

func TestFirewallHintFallsBackWhenUFWIsAbsent(t *testing.T) {
	original := ufwConfPath
	ufwConfPath = filepath.Join(t.TempDir(), "missing.conf")
	t.Cleanup(func() { ufwConfPath = original })

	info := NetworkInfo{Name: "infra_web", Gateway: "172.18.0.1", Subnet: "172.18.0.0/16"}
	hint := FirewallHint(info, "172.18.0.1:9777")
	if strings.Contains(hint, "ufw is enabled") {
		t.Errorf("hint = %q, should not claim ufw is enabled", hint)
	}
	if !strings.Contains(hint, "sudo ufw allow") {
		t.Errorf("hint = %q, want the rule as a generic suggestion", hint)
	}
}

func TestFirewallHintNeedsSubnetAndGateway(t *testing.T) {
	if hint := FirewallHint(NetworkInfo{Name: "infra_web"}, "172.18.0.1:9777"); hint != "" {
		t.Errorf("hint = %q, want empty without gateway and subnet", hint)
	}
}

func TestProbeCommandMirrorsTheProbe(t *testing.T) {
	cmd := ProbeCommand("infra_web", "172.18.0.1:9777", "")
	for _, want := range []string{"docker run --rm", "--network infra_web", probeImages[0], "http://172.18.0.1:9777/health"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command %q is missing %q", cmd, want)
		}
	}
}
