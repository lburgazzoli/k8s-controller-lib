package strcase_test

import (
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util/strcase"

	. "github.com/onsi/gomega"
)

func TestToKebabCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"already kebab", "log-level", "log-level"},
		{"camel case", "LogLevel", "log-level"},
		{"pascal case", "MaxWorkers", "max-workers"},
		{"snake case", "log_level", "log-level"},
		{"mixed", "MaxWorkers_Count", "max-workers-count"},
		{"single word", "Port", "port"},
		{"lowercase", "port", "port"},
		{"acronym", "HTTPPort", "h-t-t-p-port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result := strcase.ToKebabCase(tt.input)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"already snake", "log_level", "log_level"},
		{"camel case", "LogLevel", "log_level"},
		{"pascal case", "MaxWorkers", "max_workers"},
		{"single word", "Port", "port"},
		{"lowercase", "port", "port"},
		{"acronym", "HTTPPort", "h_t_t_p_port"},
		{"mixed", "MaxWorkersCount", "max_workers_count"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result := strcase.ToSnakeCase(tt.input)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}
