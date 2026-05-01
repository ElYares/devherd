package cli

import "testing"

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		input    string
		tld      string
		expected string
	}{
		{input: "mi-demo", tld: "test", expected: "mi-demo.test"},
		{input: "Mi Demo", tld: "test", expected: "mi-demo.test"},
		{input: "api-lab.local", tld: "test", expected: "api-lab.local"},
	}

	for _, test := range tests {
		result, err := normalizeDomain(test.input, test.tld)
		if err != nil {
			t.Fatalf("normalizeDomain(%q): %v", test.input, err)
		}

		if result != test.expected {
			t.Fatalf("normalizeDomain(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}
