.PHONY: build test cover lint vet install clean tidy run help

BIN     := bin/devherd
PKG     := ./cmd/devherd
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X github.com/devherd/devherd/internal/version.Version=$(VERSION) \
           -X github.com/devherd/devherd/internal/version.Commit=$(COMMIT) \
           -X github.com/devherd/devherd/internal/version.Date=$(DATE)

## build: compila el binario en bin/devherd con metadatos de versión
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

## install: instala devherd en $GOBIN con metadatos de versión
install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

## test: corre todos los tests con detección de carreras
test:
	go test ./... -race -count=1

## cover: genera y resume el reporte de cobertura
cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -n 1

## vet: análisis estático estándar de Go
vet:
	go vet ./...

## lint: golangci-lint (requiere golangci-lint instalado)
lint:
	golangci-lint run

## tidy: limpia go.mod/go.sum
tidy:
	go mod tidy

## run: compila y ejecuta (uso: make run ARGS="doctor")
run: build
	$(BIN) $(ARGS)

## clean: borra artefactos de build
clean:
	rm -rf bin coverage.out

## help: lista los targets disponibles
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
