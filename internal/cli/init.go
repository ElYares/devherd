package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/database"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var proxyDriver string
	var localTLD string
	var runtimeManager string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap local DevHerd directories, config and database",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}

			if err := paths.Ensure(); err != nil {
				return fmt.Errorf("create local directories: %w", err)
			}

			store := config.NewStore(paths.ConfigFile)
			cfg := config.Default()
			configCreated := false

			loaded, err := store.Load()
			switch {
			case err == nil:
				cfg = loaded
			case errors.Is(err, fs.ErrNotExist):
				configCreated = true
			default:
				return fmt.Errorf("load config: %w", err)
			}

			if err := applyInitOverrides(cmd, &cfg, proxyDriver, localTLD, runtimeManager); err != nil {
				return err
			}

			if err := store.Save(cfg); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			dbCreated, err := database.NewManager(paths.DBFile).Ensure(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "DevHerd initialized")
			fmt.Fprintf(out, "config: %s\n", paths.ConfigFile)
			fmt.Fprintf(out, "database: %s\n", paths.DBFile)
			fmt.Fprintf(out, "proxy driver: %s\n", cfg.Proxy.Driver)
			fmt.Fprintf(out, "local tld: .%s\n", cfg.LocalTLD)
			fmt.Fprintf(out, "runtime manager: %s\n", cfg.RuntimeManager)

			if configCreated {
				fmt.Fprintln(out, "config status: created")
			} else {
				fmt.Fprintln(out, "config status: reused")
			}

			if dbCreated {
				fmt.Fprintln(out, "database status: created")
			} else {
				fmt.Fprintln(out, "database status: migrated")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&proxyDriver, "proxy", "caddy", "Reverse proxy driver: caddy or nginx")
	cmd.Flags().StringVar(&localTLD, "tld", "test", "Local top-level domain")
	cmd.Flags().StringVar(&runtimeManager, "runtime-manager", "mise", "Runtime manager: mise or asdf")

	return cmd
}

func applyInitOverrides(cmd *cobra.Command, cfg *config.Config, proxyDriver, localTLD, runtimeManager string) error {
	if cmd.Flags().Changed("proxy") {
		switch proxyDriver {
		case "caddy", "nginx":
			cfg.Proxy.Driver = proxyDriver
		default:
			return fmt.Errorf("unsupported proxy driver %q", proxyDriver)
		}
	}

	if cmd.Flags().Changed("tld") {
		localTLD = strings.TrimPrefix(localTLD, ".")
		if localTLD == "" {
			return errors.New("tld cannot be empty")
		}

		cfg.LocalTLD = localTLD
	}

	if cmd.Flags().Changed("runtime-manager") {
		switch runtimeManager {
		case "mise", "asdf":
			cfg.RuntimeManager = runtimeManager
		default:
			return fmt.Errorf("unsupported runtime manager %q", runtimeManager)
		}
	}

	return nil
}
