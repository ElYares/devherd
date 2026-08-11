package observe

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSharedNetworkNamesDedupes(t *testing.T) {
	// El driver por defecto ya usa infra_web: no debe aparecer dos veces.
	names := SharedNetworkNames("infra_web")
	if len(names) != 2 || names[0] != "infra_web" || names[1] != "infra_net" {
		t.Fatalf("names = %v, want [infra_web infra_net]", names)
	}

	// Una red de proxy propia se antepone sin perder las conocidas.
	names = SharedNetworkNames("custom_web")
	if len(names) != 3 || names[0] != "custom_web" {
		t.Fatalf("names = %v, want the custom network first", names)
	}

	if got := SharedNetworkNames("  "); len(got) != 2 {
		t.Errorf("empty proxy network should fall back to the known ones, got %v", got)
	}
}

func TestInspectNetworksSkipsMissingOnes(t *testing.T) {
	// infra_web existe, infra_net no: el collector debe escuchar en la que haya.
	r := &fakeRunner{
		outputs: []string{"172.18.0.1|172.18.0.0/16", ""},
		errs:    []error{nil, errors.New("network infra_net not found")},
	}

	infos := InspectNetworks(context.Background(), r, []string{"infra_web", "infra_net"})
	if len(infos) != 1 || infos[0].Name != "infra_web" {
		t.Fatalf("infos = %+v, want only infra_web", infos)
	}
}

func TestProjectNetworkCoverageCountsPerContainer(t *testing.T) {
	// Caso real de aang: solo el servicio del proxy esta en infra_web, asi que
	// la union mentiria y la cobertura no.
	r := &fakeRunner{outputs: []string{
		"aang_app\naang_queue\naang_web\naang_db",
		"aang_net infra_net \n" +
			"aang_net infra_net \n" +
			"aang_net infra_web \n" +
			"aang_net \n",
	}}

	coverage, err := ProjectNetworkCoverage(context.Background(), r, "aang-server")
	if err != nil {
		t.Fatalf("ProjectNetworkCoverage returned error: %v", err)
	}
	if coverage.Containers != 4 {
		t.Fatalf("containers = %d, want 4", coverage.Containers)
	}
	if coverage.Networks["aang_net"] != 4 {
		t.Errorf("aang_net coverage = %d, want 4", coverage.Networks["aang_net"])
	}
	if coverage.Networks["infra_web"] != 1 {
		t.Errorf("infra_web coverage = %d, want 1", coverage.Networks["infra_web"])
	}

	if filter := strings.Join(r.calls[0], " "); !strings.Contains(filter, "label=devherd.project=aang-server") {
		t.Errorf("first call = %q, want the devherd.project filter", filter)
	}
}

func TestProjectNetworkCoverageWithoutRunningContainers(t *testing.T) {
	r := &fakeRunner{outputs: []string{"  "}}

	coverage, err := ProjectNetworkCoverage(context.Background(), r, "aang-server")
	if err != nil {
		t.Fatalf("ProjectNetworkCoverage returned error: %v", err)
	}
	if coverage.Containers != 0 {
		t.Errorf("containers = %d, want 0", coverage.Containers)
	}
	if len(r.calls) != 1 {
		t.Errorf("expected no inspect call when there are no containers, got %v", r.calls)
	}
}

func TestSelectProjectNetworkPrefersTheStableNetwork(t *testing.T) {
	// Caso aang: su red privada cubre 6/6, pero cambia de subred al recrearla.
	// infra_net cubre 2/6 y son justo los servicios que reportan.
	coverage := NetworkCoverage{
		Containers: 6,
		Networks:   map[string]int{"aang_net": 6, "infra_net": 2, "infra_web": 1},
	}

	name, count := SelectProjectNetwork(coverage, []string{"infra_web", "infra_net"})
	if name != "infra_net" || count != 2 {
		t.Fatalf("selected %q with %d, want infra_net with 2", name, count)
	}
}

func TestSelectProjectNetworkPicksTheWidestStableNetwork(t *testing.T) {
	// Caso tlmas: solo una red DevHerd lo cubre, y es la del proxy.
	coverage := NetworkCoverage{
		Containers: 4,
		Networks:   map[string]int{"tlmas_default": 4, "infra_web": 1},
	}

	if name, count := SelectProjectNetwork(coverage, []string{"infra_web", "infra_net"}); name != "infra_web" || count != 1 {
		t.Fatalf("selected %q with %d, want infra_web with 1", name, count)
	}
}

func TestSelectProjectNetworkFallsBackToTheProjectNetwork(t *testing.T) {
	// Proyecto aislado: ninguna red DevHerd lo toca, asi que no hay alternativa.
	coverage := NetworkCoverage{Containers: 3, Networks: map[string]int{"solo_net": 3, "other_net": 1}}

	for i := 0; i < 20; i++ {
		name, count := SelectProjectNetwork(coverage, []string{"infra_web", "infra_net"})
		if name != "solo_net" || count != 3 {
			t.Fatalf("selected %q with %d, want solo_net with 3", name, count)
		}
	}
}

func TestSelectProjectNetworkIsDeterministicOnTies(t *testing.T) {
	coverage := NetworkCoverage{Containers: 2, Networks: map[string]int{"zzz_net": 2, "aaa_net": 2}}

	for i := 0; i < 20; i++ {
		if name, _ := SelectProjectNetwork(coverage, nil); name != "aaa_net" {
			t.Fatalf("selected %q, want aaa_net", name)
		}
	}
}

func TestSelectProjectNetworkWithoutCoverage(t *testing.T) {
	if name, count := SelectProjectNetwork(NetworkCoverage{}, nil); name != "" || count != 0 {
		t.Errorf("selected %q with %d, want an empty result", name, count)
	}
}

func TestObservedNetworksUnion(t *testing.T) {
	r := &fakeRunner{outputs: []string{
		"aang_app\ntlmas_app",
		"aang_net infra_net \ntlmas_default infra_web \n",
	}}

	networks, err := ObservedNetworks(context.Background(), r)
	if err != nil {
		t.Fatalf("ObservedNetworks returned error: %v", err)
	}
	if len(networks) != 4 {
		t.Fatalf("networks = %v, want the four distinct ones", networks)
	}

	if filter := strings.Join(r.calls[0], " "); !strings.Contains(filter, "label=devherd.observe=true") {
		t.Errorf("first call = %q, want the devherd.observe filter", filter)
	}
}
