package observe

import (
	"context"
	"strings"
	"time"
)

type Correlator struct {
	store  Store
	docker DockerRuntime
}

func NewCorrelator(store Store, docker DockerRuntime) Correlator {
	return Correlator{store: store, docker: docker}
}

func (c Correlator) CorrelateEvent(ctx context.Context, event *Event) ([]ContainerLog, error) {
	if c.docker == nil || event == nil {
		return nil, nil
	}

	containers, err := c.docker.ObservedContainers(ctx, event.Project)
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, nil
	}

	if _, err := c.store.StoreContainers(ctx, containers); err != nil {
		return nil, err
	}

	container := bestContainerMatch(*event, containers)
	if container.ContainerID == "" {
		return nil, nil
	}

	if event.Container == "" {
		event.Container = container.Name
	}
	if event.Service == "" {
		event.Service = container.Service
	}

	eventTime := parseObserveTime(event.Timestamp)
	logs, err := c.docker.LogsAround(ctx, firstNonEmpty(container.Name, container.ContainerID), eventTime, 30*time.Second, 200)
	if err != nil {
		return nil, err
	}

	for i := range logs {
		logs[i].Project = event.Project
		logs[i].Service = firstNonEmpty(event.Service, container.Service)
		logs[i].Container = firstNonEmpty(event.Container, container.Name)
	}

	return logs, nil
}

func bestContainerMatch(event Event, containers []ObservedContainer) ObservedContainer {
	if len(containers) == 0 {
		return ObservedContainer{}
	}

	for _, container := range containers {
		if event.Container != "" && (strings.EqualFold(container.Name, event.Container) || strings.HasPrefix(container.ContainerID, event.Container)) {
			return container
		}
	}

	for _, container := range containers {
		if event.Service != "" && strings.EqualFold(container.Service, event.Service) {
			return container
		}
	}

	if len(containers) == 1 {
		return containers[0]
	}

	return ObservedContainer{}
}

func parseObserveTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().UTC()
	}

	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC()
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC()
	}

	return time.Now().UTC()
}
