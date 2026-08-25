package cli

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/database"
	"github.com/devherd/devherd/internal/detector"
	"github.com/devherd/devherd/internal/observe"
	"github.com/devherd/devherd/internal/proxy"
)

func prepareComposeProject(ctx context.Context, app *appContext, targetPath string) (compose.Project, error) {
	project, err := compose.ResolveProject(targetPath)
	if err != nil {
		return compose.Project{}, err
	}

	if app == nil || !proxy.UsesDockerExternal(app.Config) {
		return appendObserveOverride(project), nil
	}

	externalProject, err := resolveExternalProject(ctx, app, project.Root)
	if err != nil {
		// Que no se pueda resolver el proxy no tiene por que apagar Observe: son
		// dos overrides independientes, y devolver el proyecto pelado dejaba al
		// contenedor sin DSN y sin reportar, en silencio.
		slog.Debug("compose: proxy override skipped; project could not be resolved",
			"path", project.Root, "error", err)
		return appendObserveOverride(project), nil
	}

	overridePath, err := proxy.EnsureComposeOverride(app.Config, externalProject)
	if err != nil {
		return compose.Project{}, err
	}

	project.ComposeFiles = append(project.ComposeFiles, overridePath)
	return appendObserveOverride(project), nil
}

func resolveExternalProject(ctx context.Context, app *appContext, root string) (proxy.ExternalProject, error) {
	record, ok, err := database.FindProjectByPath(ctx, app.DB, root)
	if err != nil {
		return proxy.ExternalProject{}, err
	}

	if !ok {
		detected, found, err := detector.DetectProject(root)
		if err != nil {
			return proxy.ExternalProject{}, err
		}

		record = database.ProjectRecord{
			Name:      filepath.Base(root),
			Path:      root,
			Framework: detected.Framework,
			Stack:     detected.Stack,
			Runtime:   detected.Runtime,
		}
		if found {
			record.Name = detected.Name
			record.Path = detected.Path
		}
	}

	return proxy.BuildExternalProject(app.Config, record)
}

func appendObserveOverride(project compose.Project) compose.Project {
	overridePath := filepath.Join(project.Root, observe.ManagedComposeOverrideFile)
	if _, err := os.Stat(overridePath); err == nil {
		project.ComposeFiles = append(project.ComposeFiles, overridePath)
	}

	return project
}
