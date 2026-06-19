package version

import "testing"

func TestStringReturnsVersion(t *testing.T) {
	if String() != Version {
		t.Fatalf("String() = %q, want %q", String(), Version)
	}
}

func TestLongIncludesVersionCommitAndDate(t *testing.T) {
	got := Long()
	for _, want := range []string{Version, Commit, Date} {
		if want == "" {
			continue
		}
		if !contains(got, want) {
			t.Errorf("Long() = %q, missing %q", got, want)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
