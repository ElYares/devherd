package dns

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const (
	hostsPath    = "/etc/hosts"
	blockStart   = "# devherd start"
	blockEnd     = "# devherd end"
	loopbackLine = "127.0.0.1"
)

// domainPattern admite únicamente hostnames seguros (minúsculas, dígitos,
// guiones y puntos). Rechaza espacios, saltos de línea y metacaracteres que
// podrían inyectar entradas adicionales en /etc/hosts.
var domainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

// validateDomains normaliza (trim + minúsculas), deduplica y valida cada dominio.
// Devuelve error ante cualquier dominio inválido en vez de escribirlo en /etc/hosts.
func validateDomains(domains []string) ([]string, error) {
	seen := make(map[string]bool, len(domains))
	clean := make([]string, 0, len(domains))
	for _, d := range domains {
		normalized := strings.ToLower(strings.TrimSpace(d))
		if normalized == "" {
			continue
		}
		if !domainPattern.MatchString(normalized) {
			return nil, fmt.Errorf("invalid domain %q: only lowercase letters, digits, hyphens and dots are allowed", d)
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		clean = append(clean, normalized)
	}

	return clean, nil
}

func SyncHosts(domains []string) error {
	clean, err := validateDomains(domains)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("read hosts file: %w", err)
	}

	updated := mergeManagedBlock(string(content), clean)
	tempFile, err := os.CreateTemp("", "devherd-hosts-*")
	if err != nil {
		return fmt.Errorf("create temp hosts file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(updated); err != nil {
		tempFile.Close()
		return fmt.Errorf("write temp hosts file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp hosts file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "DevHerd necesita permisos de sudo para actualizar %s con %d dominio(s) administrados.\n", hostsPath, len(clean))

	if err := runInteractive("sudo", "-v"); err != nil {
		return err
	}

	if err := runInteractive("sudo", "cp", tempFile.Name(), hostsPath); err != nil {
		return fmt.Errorf("replace hosts file: %w", err)
	}

	return nil
}

func mergeManagedBlock(content string, domains []string) string {
	lines := strings.Split(content, "\n")
	var filtered []string
	inManagedBlock := false
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case blockStart:
			inManagedBlock = true
			continue
		case blockEnd:
			inManagedBlock = false
			continue
		}

		if !inManagedBlock {
			filtered = append(filtered, line)
		}
	}

	trimmed := strings.TrimRight(strings.Join(filtered, "\n"), "\n")
	managedBlock := buildManagedBlock(domains)
	if trimmed == "" {
		return managedBlock + "\n"
	}

	return trimmed + "\n\n" + managedBlock + "\n"
}

func buildManagedBlock(domains []string) string {
	if len(domains) == 0 {
		return blockStart + "\n" + blockEnd
	}

	return strings.Join([]string{
		blockStart,
		loopbackLine + " " + strings.Join(domains, " "),
		blockEnd,
	}, "\n")
}

func runInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
