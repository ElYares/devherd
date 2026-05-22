package cli

import (
	"reflect"
	"testing"
)

func TestBrowserCommand(t *testing.T) {
	url := "http://example.localhost"

	cases := []struct {
		goos string
		name string
		args []string
		ok   bool
	}{
		{goos: "linux", name: "xdg-open", args: []string{url}, ok: true},
		{goos: "darwin", name: "open", args: []string{url}, ok: true},
		{goos: "windows", name: "cmd", args: []string{"/c", "start", "", url}, ok: true},
		{goos: "plan9", name: "", args: nil, ok: false},
	}

	for _, tc := range cases {
		name, args, ok := browserCommand(tc.goos, url)
		if name != tc.name || ok != tc.ok || !reflect.DeepEqual(args, tc.args) {
			t.Fatalf("browserCommand(%q) = (%q, %#v, %t), want (%q, %#v, %t)", tc.goos, name, args, ok, tc.name, tc.args, tc.ok)
		}
	}
}
