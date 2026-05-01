package dns

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	hostsPath    = "/etc/hosts"
	blockStart   = "# devherd start"
	blockEnd     = "# devherd end"
	loopbackLine = "127.0.0.1"
)

func SyncHosts(domains []string) error {
	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("read hosts file: %w", err)
	}

	updated := mergeManagedBlock(string(content), domains)
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
