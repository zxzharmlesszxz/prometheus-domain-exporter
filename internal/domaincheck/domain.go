package domaincheck

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
)

func ParseDomains(values []string) ([]string, []string, error) {
	seen := make(map[string]struct{}, len(values))
	domains := make([]string, 0, len(values))
	var warnings []string
	for _, value := range values {
		name, err := Normalize(value)
		if err != nil {
			return nil, nil, err
		}
		if name != value {
			warnings = append(warnings, value)
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		domains = append(domains, name)
	}
	return domains, warnings, nil
}

func Normalize(value string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(value))
	name = strings.TrimSuffix(name, ".")
	if name == "" {
		return "", fmt.Errorf("domain name is empty")
	}
	if strings.Contains(name, "://") || strings.ContainsAny(name, "/:") {
		return "", fmt.Errorf("domain %q must be a domain name, not a URL or host:port", value)
	}

	if hasNonASCII(name) {
		ascii, err := idna.ToASCII(name)
		if err != nil {
			return "", fmt.Errorf("domain %q contains unsupported internationalized characters: %w", value, err)
		}
		name = ascii
	}

	if len(name) > 253 {
		return "", fmt.Errorf("domain %q is too long", value)
	}

	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("domain %q must include a top-level domain", value)
	}
	for _, label := range labels {
		if err := validateLabel(value, label); err != nil {
			return "", err
		}
	}
	return name, nil
}

func validateLabel(original string, label string) error {
	if label == "" {
		return fmt.Errorf("domain %q contains an empty label", original)
	}
	if len(label) > 63 {
		return fmt.Errorf("domain %q contains a label longer than 63 characters", original)
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return fmt.Errorf("domain %q contains a label with leading or trailing hyphen", original)
	}
	for _, r := range label {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			continue
		}
		return fmt.Errorf("domain %q contains unsupported character %q", original, r)
	}
	return nil
}

func hasNonASCII(s string) bool {
	for i := range len(s) {
		if s[i] > unicode.MaxASCII {
			return true
		}
	}
	return false
}

func tld(name string) string {
	name = strings.TrimSuffix(name, ".")
	index := strings.LastIndexByte(name, '.')
	if index < 0 {
		return name
	}
	return name[index+1:]
}
