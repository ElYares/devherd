package preflight

import "testing"

func TestExpandEnvUsesValueAndDefault(t *testing.T) {
	env := map[string]string{"APP_PORT": "8083"}

	got := expandEnv("127.0.0.1:${APP_PORT:-8080}:${MISSING:-80}", env)
	want := "127.0.0.1:8083:80"
	if got != want {
		t.Fatalf("unexpected expansion: got %q want %q", got, want)
	}
}

func TestParsePortStringWithHostIP(t *testing.T) {
	env := map[string]string{"APP_PORT": "8083"}

	port, ok := parsePort("web", "127.0.0.1:${APP_PORT:-8080}:80", env)
	if !ok {
		t.Fatal("expected port to parse")
	}

	if port.Service != "web" || port.Host != "127.0.0.1" || port.Port != 8083 {
		t.Fatalf("unexpected port: %#v", port)
	}
}

func TestParsePortStringWithoutHostIP(t *testing.T) {
	port, ok := parsePort("vite", "${VITE_PORT:-5173}:5173", map[string]string{})
	if !ok {
		t.Fatal("expected port to parse")
	}

	if port.Service != "vite" || port.Host != "" || port.Port != 5173 {
		t.Fatalf("unexpected port: %#v", port)
	}
}

func TestInspectContainerNamesAcceptsComposeNamePrefix(t *testing.T) {
	findings := inspectContainerNames([]composeDoc{
		{
			Services: map[string]serviceConfig{
				"app": {ContainerName: "${COMPOSE_NAME_PREFIX:-aang}_app"},
			},
		},
	}, map[string]string{"COMPOSE_NAME_PREFIX": "aang"}, map[string]dockerContainer{}, "aang-server")

	if len(findings) != 1 {
		t.Fatalf("unexpected finding count: %#v", findings)
	}

	if findings[0].Severity != SeverityOK {
		t.Fatalf("expected OK finding, got %#v", findings[0])
	}
}

func TestInspectContainerNamesWarnsWhenComposeNamePrefixMissing(t *testing.T) {
	findings := inspectContainerNames([]composeDoc{
		{
			Services: map[string]serviceConfig{
				"app": {ContainerName: "${COMPOSE_NAME_PREFIX:-aang}_app"},
			},
		},
	}, map[string]string{}, map[string]dockerContainer{}, "aang-server")

	if len(findings) != 1 {
		t.Fatalf("unexpected finding count: %#v", findings)
	}

	if findings[0].Severity != SeverityWarn {
		t.Fatalf("expected warn finding, got %#v", findings[0])
	}
}

func TestReportCountsWarningsAndFailures(t *testing.T) {
	report := Report{
		Findings: []Finding{
			{Severity: SeverityOK},
			{Severity: SeverityWarn},
			{Severity: SeverityWarn},
			{Severity: SeverityFail},
		},
	}

	if !report.HasWarnings() || report.Count(SeverityWarn) != 2 {
		t.Fatalf("expected two warnings, got %#v", report)
	}

	if !report.HasFailures() || report.Count(SeverityFail) != 1 {
		t.Fatalf("expected one failure, got %#v", report)
	}
}
