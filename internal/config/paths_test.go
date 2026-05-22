package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultDataRootForOS(t *testing.T) {
	home := "/home/devherd"
	configRoot := "/config/root"

	cases := []struct {
		goos string
		want string
	}{
		{goos: "linux", want: filepath.Join(home, ".local", "share")},
		{goos: "darwin", want: configRoot},
		{goos: "windows", want: configRoot},
	}

	for _, tc := range cases {
		if got := defaultDataRootForOS(tc.goos, home, configRoot); got != tc.want {
			t.Fatalf("defaultDataRootForOS(%q) = %q, want %q", tc.goos, got, tc.want)
		}
	}
}

func TestDefaultStateRootForOS(t *testing.T) {
	home := "/home/devherd"
	cacheRoot := "/cache/root"

	cases := []struct {
		goos string
		want string
	}{
		{goos: "linux", want: filepath.Join(home, ".local", "state")},
		{goos: "darwin", want: cacheRoot},
		{goos: "windows", want: cacheRoot},
	}

	for _, tc := range cases {
		if got := defaultStateRootForOS(tc.goos, home, cacheRoot); got != tc.want {
			t.Fatalf("defaultStateRootForOS(%q) = %q, want %q", tc.goos, got, tc.want)
		}
	}
}
