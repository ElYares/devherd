package cli

import (
	"context"
	"path/filepath"

	"github.com/devherd/devherd/internal/compose"
	"github.com/devherd/devherd/internal/database"
	"github.com/devherd/devherd/internal/detector"
	"github.com/devherd/devherd/internal/proxy"
)

func prepareComposeProject(ctx context.Context, app *appContext, targetPath string) (compose.Project, error) {
	project, err := compose.ResolveProject(targetPath)
	if err != nil {
		return compose.Project{}, err
	}

	if app == nil || !proxy.UsesDockerExternal(app.Config) {
		return project, nil
	}

	externalProject, err := resolveExternalProject(ctx, app, project.Root)
	if err != nil {
		return project, nil
	}

	overridePath, err := proxy.EnsureComposeOverride(externalProject)
	if err != nil {
		return compose.Project{}, err
	}

	project.ComposeFiles = append(project.ComposeFiles, overridePath)
	return project, nil
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
