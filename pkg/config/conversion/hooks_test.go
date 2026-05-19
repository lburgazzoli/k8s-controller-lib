package conversion_test

import (
	"encoding"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/config"

	. "github.com/onsi/gomega"
)

// Priority is a custom type for testing TextUnmarshaler.
type Priority int

const (
	PriorityLow Priority = iota + 1
	PriorityMedium
	PriorityHigh
)

func (p *Priority) UnmarshalText(text []byte) error {
	switch string(text) {
	case "low":
		*p = PriorityLow
	case "medium":
		*p = PriorityMedium
	case "high":
		*p = PriorityHigh
	default:
		return fmt.Errorf("invalid priority: %s", string(text))
	}

	return nil
}

var _ encoding.TextUnmarshaler = (*Priority)(nil)

func TestTimeHook(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		wantErr   bool
		checkFunc func(*WithT, time.Time)
	}{
		{
			name:    "valid RFC3339 time",
			input:   "2024-01-15T10:30:00Z",
			wantErr: false,
			checkFunc: func(g *WithT, result time.Time) {
				expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
				g.Expect(result.Equal(expected)).To(BeTrue())
			},
		},
		{
			name:    "valid RFC3339 with timezone",
			input:   "2024-01-15T10:30:00+02:00",
			wantErr: false,
			checkFunc: func(g *WithT, result time.Time) {
				g.Expect(result.Year()).To(Equal(2024))
				g.Expect(result.Month()).To(Equal(time.January))
				g.Expect(result.Day()).To(Equal(15))
			},
		},
		{
			name:    "invalid time format",
			input:   "not-a-time",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: false, // Empty env var means field stays at zero value
			checkFunc: func(g *WithT, result time.Time) {
				g.Expect(result.IsZero()).To(BeTrue())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			type TestConfig struct {
				Timestamp time.Time `mapstructure:"timestamp"`
			}

			cfg := &TestConfig{}
			loader, err := config.For(cfg)
			g.Expect(err).ToNot(HaveOccurred())

			// Test through the full config flow
			t.Setenv("CONTROLLER_TIMESTAMP", tc.input)

			err = loader.Load()

			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				if tc.checkFunc != nil {
					tc.checkFunc(g, cfg.Timestamp)
				}
			}
		})
	}
}

func TestURLHook(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*WithT, url.URL)
	}{
		{
			name:    "valid HTTP URL",
			input:   "https://example.com:8080/path?query=value",
			wantErr: false,
			check: func(g *WithT, result url.URL) {
				g.Expect(result.Scheme).To(Equal("https"))
				g.Expect(result.Host).To(Equal("example.com:8080"))
				g.Expect(result.Path).To(Equal("/path"))
				g.Expect(result.RawQuery).To(Equal("query=value"))
			},
		},
		{
			name:    "simple URL",
			input:   "http://localhost",
			wantErr: false,
			check: func(g *WithT, result url.URL) {
				g.Expect(result.Scheme).To(Equal("http"))
				g.Expect(result.Host).To(Equal("localhost"))
			},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
			check: func(g *WithT, result url.URL) {
				g.Expect(result.String()).To(Equal(""))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			type TestConfig struct {
				Endpoint url.URL `mapstructure:"endpoint"`
			}

			cfg := &TestConfig{}
			loader, err := config.For(cfg)
			g.Expect(err).ToNot(HaveOccurred())

			t.Setenv("CONTROLLER_ENDPOINT", tc.input)

			err = loader.Load()

			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				if tc.check != nil {
					tc.check(g, cfg.Endpoint)
				}
			}
		})
	}
}

func TestIPHook(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*WithT, net.IP)
	}{
		{
			name:    "valid IPv4",
			input:   "192.168.1.1",
			wantErr: false,
			check: func(g *WithT, result net.IP) {
				g.Expect(result.String()).To(Equal("192.168.1.1"))
			},
		},
		{
			name:    "valid IPv6",
			input:   "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			wantErr: false,
			check: func(g *WithT, result net.IP) {
				g.Expect(result).NotTo(BeNil())
			},
		},
		{
			name:    "invalid IP",
			input:   "not-an-ip",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: false, // Empty env var means field stays at zero value
			check: func(g *WithT, result net.IP) {
				g.Expect(result).To(BeNil())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			type TestConfig struct {
				ServerIP net.IP `mapstructure:"server_ip"`
			}

			cfg := &TestConfig{}
			loader, err := config.For(cfg)
			g.Expect(err).ToNot(HaveOccurred())

			t.Setenv("CONTROLLER_SERVER_IP", tc.input)

			err = loader.Load()

			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				if tc.check != nil {
					tc.check(g, cfg.ServerIP)
				}
			}
		})
	}
}

func TestTextUnmarshalerHook(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		wantErr  bool
		expected Priority
	}{
		{
			name:     "low priority",
			input:    "low",
			wantErr:  false,
			expected: PriorityLow,
		},
		{
			name:     "medium priority",
			input:    "medium",
			wantErr:  false,
			expected: PriorityMedium,
		},
		{
			name:     "high priority",
			input:    "high",
			wantErr:  false,
			expected: PriorityHigh,
		},
		{
			name:    "invalid priority",
			input:   "urgent",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			type TestConfig struct {
				Priority Priority `mapstructure:"priority"`
			}

			cfg := &TestConfig{}
			loader, err := config.For(cfg)
			g.Expect(err).ToNot(HaveOccurred())

			t.Setenv("CONTROLLER_PRIORITY", tc.input)

			err = loader.Load()

			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(cfg.Priority).To(Equal(tc.expected))
			}
		})
	}
}

func TestPointerHook(t *testing.T) {
	t.Run("converts to pointer type", func(t *testing.T) {
		g := NewWithT(t)

		type TestConfig struct {
			OptionalPort *int    `mapstructure:"optional_port"`
			OptionalName *string `mapstructure:"optional_name"`
		}

		cfg := &TestConfig{}
		loader, err := config.For(cfg)
		g.Expect(err).ToNot(HaveOccurred())

		t.Setenv("CONTROLLER_OPTIONAL_PORT", "8080")
		t.Setenv("CONTROLLER_OPTIONAL_NAME", "test-service")

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(cfg.OptionalPort).NotTo(BeNil())
		g.Expect(*cfg.OptionalPort).To(Equal(8080))

		g.Expect(cfg.OptionalName).NotTo(BeNil())
		g.Expect(*cfg.OptionalName).To(Equal("test-service"))
	})

	t.Run("handles nil pointer when not set", func(t *testing.T) {
		g := NewWithT(t)

		type TestConfig struct {
			OptionalValue *string `mapstructure:"optional_value"`
		}

		cfg := &TestConfig{}
		loader, err := config.For(cfg)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(cfg.OptionalValue).To(BeNil())
	})
}

func TestComposedHooks(t *testing.T) {
	t.Run("all hooks work together", func(t *testing.T) {
		g := NewWithT(t)

		type TestConfig struct {
			Name      string        `mapstructure:"name"`
			Port      int           `mapstructure:"port"`
			Timeout   time.Duration `mapstructure:"timeout"`
			StartTime time.Time     `mapstructure:"start_time"`
			Endpoint  url.URL       `mapstructure:"endpoint"`
			ServerIP  net.IP        `mapstructure:"server_ip"`
			Priority  Priority      `mapstructure:"priority"`
			Optional  *string       `mapstructure:"optional"`
		}

		cfg := &TestConfig{}
		loader, err := config.For(cfg)
		g.Expect(err).ToNot(HaveOccurred())

		t.Setenv("CONTROLLER_NAME", "test")
		t.Setenv("CONTROLLER_PORT", "8080")
		t.Setenv("CONTROLLER_TIMEOUT", "30s")
		t.Setenv("CONTROLLER_START_TIME", "2024-01-15T10:30:00Z")
		t.Setenv("CONTROLLER_ENDPOINT", "https://example.com")
		t.Setenv("CONTROLLER_SERVER_IP", "192.168.1.1")
		t.Setenv("CONTROLLER_PRIORITY", "high")
		t.Setenv("CONTROLLER_OPTIONAL", "present")

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(cfg).To(Equal(&TestConfig{
			Name:      "test",
			Port:      8080,
			Timeout:   30 * time.Second,
			StartTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			Endpoint: url.URL{
				Scheme: "https",
				Host:   "example.com",
			},
			ServerIP: net.ParseIP("192.168.1.1"),
			Priority: PriorityHigh,
			Optional: new("present"),
		}))
	})
}

func TestTimeFormatsInHook(t *testing.T) {
	t.Run("supports multiple time formats", func(t *testing.T) {
		testCases := []struct {
			name      string
			input     string
			formats   []string
			wantErr   bool
			checkFunc func(*WithT, time.Time)
		}{
			{
				name:    "RFC3339",
				input:   "2024-01-15T10:30:00Z",
				formats: []string{time.RFC3339},
				wantErr: false,
				checkFunc: func(g *WithT, result time.Time) {
					g.Expect(result.Year()).To(Equal(2024))
					g.Expect(result.Month()).To(Equal(time.January))
					g.Expect(result.Day()).To(Equal(15))
				},
			},
			{
				name:    "Date only",
				input:   "2024-01-15",
				formats: []string{"2006-01-02"},
				wantErr: false,
				checkFunc: func(g *WithT, result time.Time) {
					g.Expect(result.Year()).To(Equal(2024))
					g.Expect(result.Month()).To(Equal(time.January))
					g.Expect(result.Day()).To(Equal(15))
				},
			},
			{
				name:    "DateTime format",
				input:   "2024-01-15 10:30:00",
				formats: []string{"2006-01-02 15:04:05"},
				wantErr: false,
				checkFunc: func(g *WithT, result time.Time) {
					g.Expect(result.Hour()).To(Equal(10))
					g.Expect(result.Minute()).To(Equal(30))
				},
			},
			{
				name:    "Try multiple formats - first succeeds",
				input:   "2024-01-15",
				formats: []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05"},
				wantErr: false,
				checkFunc: func(g *WithT, result time.Time) {
					g.Expect(result.Day()).To(Equal(15))
				},
			},
			{
				name:    "Try multiple formats - second succeeds",
				input:   "2024-01-15T10:30:00Z",
				formats: []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05"},
				wantErr: false,
				checkFunc: func(g *WithT, result time.Time) {
					g.Expect(result.Hour()).To(Equal(10))
				},
			},
			{
				name:    "No format matches",
				input:   "15/01/2024",
				formats: []string{"2006-01-02", time.RFC3339},
				wantErr: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				g := NewWithT(t)

				type TestConfig struct {
					Timestamp time.Time `mapstructure:"timestamp"`
				}

				cfg := &TestConfig{}
				t.Setenv("CONTROLLER_TIMESTAMP", tc.input)

				loader, err := config.For(cfg, config.WithTimeFormats(tc.formats...))
				g.Expect(err).ToNot(HaveOccurred())

				err = loader.Load()

				if tc.wantErr {
					g.Expect(err).To(HaveOccurred())
				} else {
					g.Expect(err).ToNot(HaveOccurred())
					if tc.checkFunc != nil {
						tc.checkFunc(g, cfg.Timestamp)
					}
				}
			})
		}
	})
}
