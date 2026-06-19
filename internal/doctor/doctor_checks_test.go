package doctor

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

// swapRunCmd sustituye el seam de ejecución por un doble durante el test.
func swapRunCmd(t *testing.T, fn func(context.Context, string, ...string) (string, error)) {
	t.Helper()
	orig := runCmd
	runCmd = fn
	t.Cleanup(func() { runCmd = orig })
}

func constRunCmd(out string, err error) func(context.Context, string, ...string) (string, error) {
	return func(context.Context, string, ...string) (string, error) { return out, err }
}

func TestCheckDockerNetwork(t *testing.T) {
	cases := []struct {
		name       string
		network    string
		out        string
		err        error
		wantStatus Status
	}{
		{"sin configurar", "", "", nil, StatusFail},
		{"bridge local ok", "infra_web", "bridge\tlocal\tfalse", nil, StatusOK},
		{"driver no bridge", "infra_web", "overlay\tlocal\tfalse", nil, StatusWarn},
		{"scope no local", "infra_web", "bridge\tswarm\tfalse", nil, StatusWarn},
		{"red interna", "infra_web", "bridge\tlocal\ttrue", nil, StatusWarn},
		{"red ausente", "infra_web", "", errors.New("no such network"), StatusWarn},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			swapRunCmd(t, constRunCmd(tc.out, tc.err))
			got := checkDockerNetwork(context.Background(), tc.network, "proxy network")
			if got.Status != tc.wantStatus {
				t.Fatalf("status = %q (%s), want %q", got.Status, got.Message, tc.wantStatus)
			}
		})
	}
}

func TestCheckExternalProxyPortRequiresContainerName(t *testing.T) {
	got := checkExternalProxyPort(context.Background(), "")
	if got.Status != StatusFail {
		t.Fatalf("status = %q (%s), want fail", got.Status, got.Message)
	}
}

func TestCheckDockerDaemon(t *testing.T) {
	requireDocker(t)

	t.Run("reachable", func(t *testing.T) {
		swapRunCmd(t, constRunCmd("27.5.1", nil))
		got := checkDockerDaemon(context.Background())
		if got.Status != StatusOK || got.Message != "server 27.5.1" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("info falla", func(t *testing.T) {
		swapRunCmd(t, constRunCmd("", errors.New("cannot connect")))
		got := checkDockerDaemon(context.Background())
		if got.Status != StatusFail {
			t.Fatalf("got %+v, want fail", got)
		}
	})
}

func TestCheckDockerEngineMode(t *testing.T) {
	requireDocker(t)

	cases := []struct {
		name       string
		out        string
		wantStatus Status
	}{
		{"linux ok", "linux\tUbuntu 24.04\tnode1", StatusOK},
		{"windows fail", "windows\tWindows\twin", StatusFail},
		{"indeterminado warn", "\t\t", StatusWarn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			swapRunCmd(t, constRunCmd(tc.out, nil))
			got := checkDockerEngineMode(context.Background())
			if got.Status != tc.wantStatus {
				t.Fatalf("status = %q (%s), want %q", got.Status, got.Message, tc.wantStatus)
			}
		})
	}
}

func TestCheckDockerCompose(t *testing.T) {
	requireDocker(t)

	swapRunCmd(t, constRunCmd("Docker Compose version v2.39.1", nil))
	got := checkDockerCompose(context.Background())
	if got.Status != StatusOK {
		t.Fatalf("got %+v, want ok", got)
	}
}

func requireDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker no está en PATH; el check hace LookPath antes del seam")
	}
}
