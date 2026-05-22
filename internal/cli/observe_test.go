package cli

import "testing"

func TestObserveDSN(t *testing.T) {
	got := observeDSN("127.0.0.1:9777", "aang-server")
	want := "http://devherd@127.0.0.1:9777/aang-server"
	if got != want {
		t.Fatalf("observeDSN() = %q, want %q", got, want)
	}
}
