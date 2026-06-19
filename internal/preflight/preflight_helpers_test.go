package preflight

import "testing"

func TestSeverityRankOrdering(t *testing.T) {
	if !(severityRank(SeverityFail) > severityRank(SeverityWarn) && severityRank(SeverityWarn) > severityRank(SeverityOK)) {
		t.Fatalf("expected fail > warn > ok, got %d %d %d",
			severityRank(SeverityFail), severityRank(SeverityWarn), severityRank(SeverityOK))
	}
}

func TestHostPrefix(t *testing.T) {
	if got := hostPrefix(""); got != "" {
		t.Errorf("hostPrefix(\"\") = %q, want empty", got)
	}
	if got := hostPrefix("127.0.0.1"); got != "127.0.0.1:" {
		t.Errorf("hostPrefix = %q, want 127.0.0.1:", got)
	}
}

func TestIsExternal(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"map external true", map[string]any{"external": true}, true},
		{"map external false", map[string]any{"external": false}, false},
		{"nil", nil, false},
		{"string", "yes", false},
	}
	for _, tc := range cases {
		if got := isExternal(tc.value); got != tc.want {
			t.Errorf("%s: isExternal = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestServiceUsesNetwork(t *testing.T) {
	if !serviceUsesNetwork([]any{"default", "infra_net"}, "infra_net") {
		t.Error("expected slice form to match infra_net")
	}
	if !serviceUsesNetwork(map[string]any{"infra_net": nil}, "infra_net") {
		t.Error("expected map form to match infra_net")
	}
	if serviceUsesNetwork([]any{"default"}, "infra_net") {
		t.Error("did not expect a match")
	}
	if serviceUsesNetwork(nil, "infra_net") {
		t.Error("nil networks should not match")
	}
}

func TestUsesRedis(t *testing.T) {
	if !usesRedis(map[string]string{"CACHE_STORE": "redis"}) {
		t.Error("CACHE_STORE=redis should be detected")
	}
	if !usesRedis(map[string]string{"REDIS_HOST": "redis"}) {
		t.Error("REDIS_HOST=redis should be detected")
	}
	if usesRedis(map[string]string{"CACHE_STORE": "file"}) {
		t.Error("file cache should not be detected as redis")
	}
}

func TestPortOwner(t *testing.T) {
	containers := map[string]dockerContainer{
		"a": {Name: "uniformes_db", Project: "devherd-uniformes", Ports: "0.0.0.0:3307->3306/tcp"},
	}
	owner, ok := portOwner(3307, containers)
	if !ok || owner.Name != "uniformes_db" {
		t.Fatalf("portOwner(3307) = %+v, %v", owner, ok)
	}
	if _, ok := portOwner(9999, containers); ok {
		t.Error("portOwner(9999) should not be owned")
	}
}

func TestInspectVolumes(t *testing.T) {
	docs := []composeDoc{{
		Volumes: map[string]volumeConfig{
			"data":  {External: true},
			"named": {External: true, Name: "shared_named"},
			"local": {External: false},
		},
	}}
	findings := inspectVolumes(docs)
	if len(findings) != 2 {
		t.Fatalf("expected 2 warnings for external volumes, got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Severity != SeverityWarn {
			t.Errorf("external volume finding should be warn, got %q", f.Severity)
		}
	}
}

func TestInspectSharedNetworks(t *testing.T) {
	docs := []composeDoc{{
		Services: map[string]serviceConfig{
			"app": {Networks: []any{"infra_net"}},
		},
	}}

	// Sin redis en el env → sin hallazgos.
	if got := inspectSharedNetworks(docs, map[string]string{}); got != nil {
		t.Errorf("expected nil without redis, got %+v", got)
	}

	// Con redis + servicio en infra_net → warn.
	got := inspectSharedNetworks(docs, map[string]string{"CACHE_STORE": "redis"})
	if len(got) != 1 || got[0].Severity != SeverityWarn {
		t.Fatalf("expected one warn finding, got %+v", got)
	}
}

func TestCollectPorts(t *testing.T) {
	docs := []composeDoc{{
		Services: map[string]serviceConfig{
			"web": {Ports: []any{"8080:80"}},
			"db":  {Ports: []any{"127.0.0.1:5432:5432"}},
		},
	}}

	ports := collectPorts(docs, map[string]string{})
	if len(ports) != 2 {
		t.Fatalf("expected 2 collected ports, got %d: %+v", len(ports), ports)
	}

	byService := map[string]servicePort{}
	for _, p := range ports {
		byService[p.Service] = p
	}
	if byService["web"].Port != 8080 {
		t.Errorf("web host port = %d, want 8080", byService["web"].Port)
	}
	if byService["db"].Host != "127.0.0.1" || byService["db"].Port != 5432 {
		t.Errorf("db port = %+v, want 127.0.0.1:5432", byService["db"])
	}
}
