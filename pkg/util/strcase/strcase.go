package strcase

import "strings"

const (
	lowercaseOffset = 32
	growthFactor    = 5
)

// ToKebabCase converts CamelCase or snake_case to kebab-case.
func ToKebabCase(s string) string {
	if s == "" {
		return s
	}

	var result strings.Builder
	result.Grow(len(s) + growthFactor)

	lastWasHyphen := false

	for i, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			// Add hyphen before uppercase if not first character and last wasn't already a hyphen
			if i > 0 && !lastWasHyphen {
				_, _ = result.WriteRune('-')
			}
			// Convert to lowercase
			_, _ = result.WriteRune(r + lowercaseOffset)
			lastWasHyphen = false
		case r == '_':
			// Replace underscore with hyphen
			_, _ = result.WriteRune('-')
			lastWasHyphen = true
		default:
			_, _ = result.WriteRune(r)
			lastWasHyphen = r == '-'
		}
	}

	return result.String()
}

// ToSnakeCase converts CamelCase to snake_case.
func ToSnakeCase(s string) string {
	if s == "" {
		return s
	}

	var result strings.Builder
	result.Grow(len(s) + growthFactor)

	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				_, _ = result.WriteRune('_')
			}
			_, _ = result.WriteRune(r + lowercaseOffset)
		} else {
			_, _ = result.WriteRune(r)
		}
	}

	return result.String()
}
