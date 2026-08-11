package observe

import (
	"strings"
	"testing"
)

func TestFirewallRulesOnePerNetwork(t *testing.T) {
	// Cada red tiene su propia subred de origen y su propio gateway de destino:
	// una sola regla no cubre a un proyecto que vive en otra red.
	infos := []NetworkInfo{
		{Name: "infra_web", Gateway: "172.18.0.1", Subnet: "172.18.0.0/16"},
		{Name: "infra_net", Gateway: "172.20.0.1", Subnet: "172.20.0.0/16"},
	}

	rules := FirewallRules(infos, "172.18.0.1:9777")
	if len(rules) != 2 {
		t.Fatalf("expected one rule per network, got %d", len(rules))
	}

	if rules[1].Subnet != "172.20.0.0/16" || rules[1].Gateway != "172.20.0.1" {
		t.Errorf("second rule = %+v, want the infra_net pair", rules[1])
	}
	if rules[0].Port != "9777" {
		t.Errorf("port = %q, want 9777", rules[0].Port)
	}

	command := rules[1].Command()
	for _, want := range []string{"sudo ufw allow", "from 172.20.0.0/16", "to 172.20.0.1", "port 9777", "proto tcp", "infra_net"} {
		if !strings.Contains(command, want) {
			t.Errorf("command %q is missing %q", command, want)
		}
	}
}

func TestFirewallRulesSkipsIncompleteNetworks(t *testing.T) {
	infos := []NetworkInfo{
		{Name: "broken"},
		{Name: "infra_web", Gateway: "172.18.0.1", Subnet: "172.18.0.0/16"},
	}

	rules := FirewallRules(infos, "172.18.0.1:9777")
	if len(rules) != 1 || rules[0].Network != "infra_web" {
		t.Fatalf("expected only the complete network, got %+v", rules)
	}
}

func TestFirewallRulesFallBackToDefaultPort(t *testing.T) {
	infos := []NetworkInfo{{Name: "infra_web", Gateway: "172.18.0.1", Subnet: "172.18.0.0/16"}}

	rules := FirewallRules(infos, "172.18.0.1")
	if len(rules) != 1 || rules[0].Port != "9777" {
		t.Fatalf("expected the default port, got %+v", rules)
	}
}
