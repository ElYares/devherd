package observe

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devherd/devherd/internal/compose"
	"gopkg.in/yaml.v3"
)

const ManagedComposeOverrideFile = ".devherd.observe.override.yml"

var supportedAttachStacks = map[string]struct{}{
	"docker":  {},
	"generic": {},
	"go":      {},
	"laravel": {},
	"node":    {},
	"python":  {},
}

type AttachOptions struct {
	ProjectName string
	Stack       string
	Services    []string
	DSN         string
	Environment string
}

type AttachResult struct {
	Path     string
	Services []string
	Content  string
}

type composeServicesDoc struct {
	Services map[string]any `yaml:"services"`
}

type observeOverrideDoc struct {
	Services map[string]observeOverrideService `yaml:"services"`
}

type observeOverrideService struct {
	Environment map[string]string `yaml:"environment"`
	Labels      map[string]string `yaml:"labels"`
}

func EnsureComposeOverride(project compose.Project, options AttachOptions) (AttachResult, error) {
	result, err := BuildComposeOverride(project, options)
	if err != nil {
		return AttachResult{}, err
	}

	if err := os.WriteFile(result.Path, []byte(result.Content), 0o644); err != nil {
		return AttachResult{}, fmt.Errorf("write observe compose override: %w", err)
	}

	return result, nil
}

func BuildComposeOverride(project compose.Project, options AttachOptions) (AttachResult, error) {
	if project.Root == "" {
		return AttachResult{}, fmt.Errorf("project root is required")
	}

	options.ProjectName = strings.TrimSpace(options.ProjectName)
	if options.ProjectName == "" {
		options.ProjectName = filepath.Base(project.Root)
	}

	options.Stack = strings.ToLower(strings.TrimSpace(options.Stack))
	if options.Stack == "" {
		return AttachResult{}, fmt.Errorf("stack is required")
	}
	if _, ok := supportedAttachStacks[options.Stack]; !ok {
		return AttachResult{}, fmt.Errorf("unsupported observe stack %q; supported stacks: docker, generic, go, laravel, node, python", options.Stack)
	}

	options.DSN = strings.TrimSpace(options.DSN)
	if options.DSN == "" {
		return AttachResult{}, fmt.Errorf("dsn is required")
	}

	options.Environment = strings.TrimSpace(options.Environment)
	if options.Environment == "" {
		options.Environment = "local"
	}

	availableServices, err := ComposeServices(project.ComposeFiles)
	if err != nil {
		return AttachResult{}, err
	}
	if len(availableServices) == 0 {
		return AttachResult{}, fmt.Errorf("no services found in compose files")
	}

	selectedServices, err := selectAttachServices(availableServices, options.Services)
	if err != nil {
		return AttachResult{}, err
	}

	doc := observeOverrideDoc{Services: map[string]observeOverrideService{}}
	for _, service := range selectedServices {
		doc.Services[service] = observeOverrideService{
			Environment: map[string]string{
				"SENTRY_DSN":            options.DSN,
				"SENTRY_ENVIRONMENT":    options.Environment,
				"DEVHERD_OBSERVE":       "1",
				"DEVHERD_PROJECT":       options.ProjectName,
				"DEVHERD_OBSERVE_STACK": options.Stack,
			},
			Labels: map[string]string{
				"devherd.observe": "true",
				"devherd.project": options.ProjectName,
				"devherd.service": service,
				"devherd.stack":   options.Stack,
			},
		}
	}

	payload, err := yaml.Marshal(doc)
	if err != nil {
		return AttachResult{}, fmt.Errorf("render observe compose override: %w", err)
	}

	content := "# Managed by DevHerd Observe. Do not commit this file.\n" + string(payload)
	return AttachResult{
		Path:     filepath.Join(project.Root, ManagedComposeOverrideFile),
		Services: selectedServices,
		Content:  content,
	}, nil
}

func ComposeServices(files []string) ([]string, error) {
	seen := map[string]struct{}{}
	for _, file := range files {
		payload, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read compose file %s: %w", file, err)
		}

		var doc composeServicesDoc
		if err := yaml.Unmarshal(payload, &doc); err != nil {
			return nil, fmt.Errorf("parse compose file %s: %w", file, err)
		}

		for service := range doc.Services {
			if service != "" {
				seen[service] = struct{}{}
			}
		}
	}

	services := make([]string, 0, len(seen))
	for service := range seen {
		services = append(services, service)
	}
	sort.Strings(services)
	return services, nil
}

func selectAttachServices(available []string, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return available, nil
	}

	availableSet := make(map[string]struct{}, len(available))
	for _, service := range available {
		availableSet[service] = struct{}{}
	}

	selectedSet := map[string]struct{}{}
	for _, service := range requested {
		service = strings.TrimSpace(service)
		if service == "" {
			continue
		}

		if _, ok := availableSet[service]; !ok {
			return nil, fmt.Errorf("service %q was not found in compose files; available services: %s", service, strings.Join(available, ", "))
		}
		selectedSet[service] = struct{}{}
	}

	selected := make([]string, 0, len(selectedSet))
	for service := range selectedSet {
		selected = append(selected, service)
	}
	sort.Strings(selected)
	if len(selected) == 0 {
		return nil, fmt.Errorf("at least one service is required")
	}

	return selected, nil
}

func RemoveComposeOverride(projectRoot string) (string, bool, error) {
	path := filepath.Join(projectRoot, ManagedComposeOverrideFile)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return path, false, nil
		}
		return path, false, fmt.Errorf("remove observe compose override: %w", err)
	}

	return path, true, nil
}
