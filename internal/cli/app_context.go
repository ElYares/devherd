package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/devherd/devherd/internal/config"
	"github.com/devherd/devherd/internal/database"
)

type appContext struct {
	Paths  config.Paths
	Config config.Config
	DB     *sql.DB
}

func loadAppContext(ctx context.Context) (*appContext, error) {
	paths, err := config.ResolvePaths()
	if err != nil {
		return nil, err
	}

	if err := paths.Ensure(); err != nil {
		return nil, fmt.Errorf("create local directories: %w", err)
	}

	store := config.NewStore(paths.ConfigFile)
	cfg, err := store.Load()
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("DevHerd is not initialized. Run `devherd init` first")
	}
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	cfg.ApplyPathDefaults(paths)

	manager := database.NewManager(paths.DBFile)
	if _, err := manager.Ensure(ctx); err != nil {
		return nil, err
	}

	db, err := manager.Open()
	if err != nil {
		return nil, err
	}

	return &appContext{
		Paths:  paths,
		Config: cfg,
		DB:     db,
	}, nil
}
