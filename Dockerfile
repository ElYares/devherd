# syntax=docker/dockerfile:1

# Build multi-stage CGO-free (modernc.org/sqlite es Go puro).
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w \
      -X github.com/devherd/devherd/internal/version.Version=${VERSION} \
      -X github.com/devherd/devherd/internal/version.Commit=${COMMIT} \
      -X github.com/devherd/devherd/internal/version.Date=${DATE}" \
    -o /out/devherd ./cmd/devherd

# Imagen final mínima. Nota: devherd orquesta Docker en el host; para usarlo
# dentro de un contenedor monta el socket de Docker (-v /var/run/docker.sock).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/devherd /usr/local/bin/devherd
ENTRYPOINT ["devherd"]
