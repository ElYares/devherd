package observe

import (
	"strings"
	"testing"
)

func TestParseObservedContainers(t *testing.T) {
	payload := `[
  {
    "Id": "abcdef123456",
    "Name": "/demo_web_1",
    "RestartCount": 2,
    "Config": {
      "Image": "nginx:alpine",
      "Labels": {
        "devherd.observe": "true",
        "devherd.project": "demo",
        "devherd.service": "web"
      }
    },
    "State": {
      "Status": "running"
    }
  }
]`

	containers, err := parseObservedContainers(payload)
	if err != nil {
		t.Fatalf("parseObservedContainers returned error: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected one container, got %d", len(containers))
	}

	container := containers[0]
	if container.Name != "demo_web_1" || container.Project != "demo" || container.Service != "web" || container.Status != "running" || container.RestartCount != 2 {
		t.Fatalf("unexpected container: %#v", container)
	}
}

func TestParseDockerLogs(t *testing.T) {
	logs := parseDockerLogs("2026-05-22T10:00:00.000000000Z first line\nsecond line without timestamp\n")
	if len(logs) != 2 {
		t.Fatalf("expected two logs, got %d", len(logs))
	}
	if logs[0].Timestamp != "2026-05-22T10:00:00.000000000Z" || logs[0].Message != "first line" {
		t.Fatalf("unexpected first log: %#v", logs[0])
	}
	if logs[1].Timestamp != "" || !strings.Contains(logs[1].Message, "second line without timestamp") {
		t.Fatalf("unexpected second log: %#v", logs[1])
	}
}
