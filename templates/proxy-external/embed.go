package proxyexternal

import "embed"

const (
	DockerComposeTemplate = "docker-compose.yml.tmpl"
	CaddyfileTemplate     = "Caddyfile.tmpl"
	EnvExampleTemplate    = ".env.example"
)

// Files contains the portable external proxy templates bundled with DevHerd.
//
//go:embed docker-compose.yml.tmpl Caddyfile.tmpl .env.example
var Files embed.FS
