package conv_test

import (
	"math"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util/conv"
)

func TestSafeInt32(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected int32
	}{
		{
			name:     "zero",
			input:    0,
			expected: 0,
		},
		{
			name:     "positive within range",
			input:    42,
			expected: 42,
		},
		{
			name:     "negative within range",
			input:    -42,
			expected: -42,
		},
		{
			name:     "max int32",
			input:    math.MaxInt32,
			expected: math.MaxInt32,
		},
		{
			name:     "min int32",
			input:    math.MinInt32,
			expected: math.MinInt32,
		},
		{
			name:     "overflow positive",
			input:    int64(math.MaxInt32) + 1,
			expected: math.MaxInt32,
		},
		{
			name:     "overflow negative",
			input:    int64(math.MinInt32) - 1,
			expected: math.MinInt32,
		},
		{
			name:     "large overflow positive",
			input:    math.MaxInt64,
			expected: math.MaxInt32,
		},
		{
			name:     "large overflow negative",
			input:    math.MinInt64,
			expected: math.MinInt32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result := conv.SafeInt32(tt.input)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestSafeUint32(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected uint32
	}{
		{
			name:     "zero",
			input:    0,
			expected: 0,
		},
		{
			name:     "within range",
			input:    42,
			expected: 42,
		},
		{
			name:     "max uint32",
			input:    math.MaxUint32,
			expected: math.MaxUint32,
		},
		{
			name:     "overflow",
			input:    uint64(math.MaxUint32) + 1,
			expected: math.MaxUint32,
		},
		{
			name:     "large overflow",
			input:    math.MaxUint64,
			expected: math.MaxUint32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result := conv.SafeUint32(tt.input)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

