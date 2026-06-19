package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func newTestRoot() (*cobra.Command, *cobra.Command) {
	root := &cobra.Command{Use: "devherd"}
	parent := &cobra.Command{Use: "serve"}
	root.AddCommand(parent)
	parent.SetContext(context.Background())
	return root, parent
}

func TestRunSiblingCommandInvokesTargetWithArgs(t *testing.T) {
	root, parent := newTestRoot()

	var gotArgs []string
	root.AddCommand(&cobra.Command{
		Use:  "up [path]",
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			gotArgs = args
			return nil
		},
	})

	if err := runSiblingCommand(parent, []string{"up", "/tmp/app"}); err != nil {
		t.Fatalf("runSiblingCommand: %v", err)
	}
	if len(gotArgs) != 1 || gotArgs[0] != "/tmp/app" {
		t.Fatalf("target received args %v, want [/tmp/app]", gotArgs)
	}
}

func TestRunSiblingCommandResolvesSubcommand(t *testing.T) {
	root, parent := newTestRoot()

	called := false
	proxyCmd := &cobra.Command{Use: "proxy"}
	proxyCmd.AddCommand(&cobra.Command{
		Use: "apply",
		RunE: func(_ *cobra.Command, _ []string) error {
			called = true
			return nil
		},
	})
	root.AddCommand(proxyCmd)

	if err := runSiblingCommand(parent, []string{"proxy", "apply"}); err != nil {
		t.Fatalf("runSiblingCommand: %v", err)
	}
	if !called {
		t.Fatal("expected proxy apply to be invoked")
	}
}

func TestRunSiblingCommandPropagatesError(t *testing.T) {
	root, parent := newTestRoot()

	root.AddCommand(&cobra.Command{
		Use:  "up",
		RunE: func(_ *cobra.Command, _ []string) error { return errors.New("boom") },
	})

	if err := runSiblingCommand(parent, []string{"up"}); err == nil {
		t.Fatal("expected error to propagate")
	}
}
