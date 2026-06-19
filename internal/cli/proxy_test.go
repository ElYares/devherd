package cli

import (
	"reflect"
	"testing"

	"github.com/devherd/devherd/internal/database"
)

func TestSyncManagedDomainsUsesCollectedDomains(t *testing.T) {
	original := syncHosts
	t.Cleanup(func() {
		syncHosts = original
	})

	var got []string
	syncHosts = func(domains []string) error {
		got = append([]string(nil), domains...)
		return nil
	}

	projects := []database.ProjectRecord{
		{Domain: "docmost.local"},
		{Domain: ""},
		{Domain: "vikunja.localhost"},
	}

	if err := syncManagedDomains(projects); err != nil {
		t.Fatalf("sync managed domains: %v", err)
	}

	want := []string{"docmost.local", "vikunja.localhost"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected synced domains: got %v want %v", got, want)
	}
}
