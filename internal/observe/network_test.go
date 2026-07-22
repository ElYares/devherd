package observe

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunner registra las invocaciones y devuelve respuestas predefinidas, una
// por llamada, igual que el doble de internal/services.
type fakeRunner struct {
	calls   [][]string
	outputs []string
	errs    []error
	idx     int
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))

	var out string
	var err error
	if f.idx < len(f.outputs) {
		out = f.outputs[f.idx]
	}
	if f.idx < len(f.errs) {
		err = f.errs[f.idx]
	}
	f.idx++

	return out, err
}

func TestInspectNetworkReturnsGatewayAndSubnet(t *testing.T) {
	r := &fakeRunner{outputs: []string{"172.18.0.1|172.18.0.0/16 "}}

	info, err := InspectNetwork(context.Background(), r, "infra_web")
	if err != nil {
		t.Fatalf("InspectNetwork returned error: %v", err)
	}
	if info.Gateway != "172.18.0.1" {
		t.Errorf("gateway = %q, want 172.18.0.1", info.Gateway)
	}
	if info.Subnet != "172.18.0.0/16" {
		t.Errorf("subnet = %q, want 172.18.0.0/16", info.Subnet)
	}
	if info.Name != "infra_web" {
		t.Errorf("name = %q, want infra_web", info.Name)
	}

	call := strings.Join(r.calls[0], " ")
	if !strings.Contains(call, "docker network inspect") || !strings.HasSuffix(call, "infra_web") {
		t.Errorf("unexpected docker call: %q", call)
	}
}

func TestInspectNetworkSkipsIPv6Gateways(t *testing.T) {
	// El collector solo escucha IPv4 y el DSN se construye sin corchetes, asi
	// que una red dual stack tiene que resolver igualmente a la IPv4.
	r := &fakeRunner{outputs: []string{"fd00::1|fd00::/64 172.20.0.1|172.20.0.0/16"}}

	info, err := InspectNetwork(context.Background(), r, "infra_web")
	if err != nil {
		t.Fatalf("InspectNetwork returned error: %v", err)
	}
	if info.Gateway != "172.20.0.1" {
		t.Errorf("gateway = %q, want 172.20.0.1", info.Gateway)
	}
}

func TestInspectNetworkFailsWithoutIPv4Gateway(t *testing.T) {
	r := &fakeRunner{outputs: []string{"fd00::1|fd00::/64"}}

	if _, err := InspectNetwork(context.Background(), r, "infra_web"); err == nil {
		t.Fatal("expected an error when the network has no IPv4 gateway")
	}
}

func TestInspectNetworkPropagatesDockerFailure(t *testing.T) {
	r := &fakeRunner{errs: []error{errors.New("network infra_web not found")}}

	if _, err := InspectNetwork(context.Background(), r, "infra_web"); err == nil {
		t.Fatal("expected an error when docker fails")
	}
}

func TestInspectNetworkRejectsEmptyName(t *testing.T) {
	if _, err := InspectNetwork(context.Background(), &fakeRunner{}, "  "); err == nil {
		t.Fatal("expected an error for an empty network name")
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:9777", true},
		{"127.5.5.5:9777", true},
		{"localhost:9777", true},
		{"[::1]:9777", true},
		{"http://devherd@127.0.0.1:9777/tl-mas-server", true},
		{"172.18.0.1:9777", false},
		{"http://devherd@172.18.0.1:9777/tl-mas-server", false},
		{"collector.local:9777", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := IsLoopbackAddr(tc.addr); got != tc.want {
			t.Errorf("IsLoopbackAddr(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestWithHostKeepsPort(t *testing.T) {
	if got := WithHost("127.0.0.1:9777", "172.18.0.1"); got != "172.18.0.1:9777" {
		t.Errorf("WithHost = %q, want 172.18.0.1:9777", got)
	}

	// Sin puerto se cae al del collector por defecto.
	if got := WithHost("127.0.0.1", "172.18.0.1"); got != "172.18.0.1:9777" {
		t.Errorf("WithHost without port = %q, want 172.18.0.1:9777", got)
	}
}
