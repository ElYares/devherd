package cli

import (
	"errors"
	"strings"
	"unicode"
)

func primaryDomainLabel(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	var builder strings.Builder
	lastHyphen := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			builder.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			builder.WriteRune('-')
			lastHyphen = true
		}
	}

	label := strings.Trim(builder.String(), "-")
	if label == "" {
		return "project"
	}

	return label
}

func primaryDomain(name, tld string) string {
	return primaryDomainLabel(name) + "." + strings.TrimPrefix(tld, ".")
}

func normalizeDomain(input, tld string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(input))
	if value == "" {
		return "", errors.New("domain cannot be empty")
	}

	if !strings.Contains(value, ".") {
		return primaryDomain(value, tld), nil
	}

	rawLabels := strings.Split(value, ".")
	labels := make([]string, 0, len(rawLabels))
	for _, rawLabel := range rawLabels {
		label := primaryDomainLabel(rawLabel)
		if label == "" {
			return "", errors.New("domain contains an empty label")
		}

		labels = append(labels, label)
	}

	if len(labels) < 2 {
		return "", errors.New("domain must contain at least one subdomain and a TLD")
	}

	return strings.Join(labels, "."), nil
}
