package zap_test

import (
	"testing"

	"github.com/spf13/pflag"
	"go.uber.org/zap/zapcore"

	configzap "github.com/lburgazzoli/k8s-controller-lib/pkg/config/zap"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func TestConfigToOptions(t *testing.T) {
	g := NewWithT(t)

	cfg := &configzap.Config{
		Development:     true,
		Level:           "debug",
		StacktraceLevel: "warn",
		Encoder:         "console",
		TimeEncoding:    "iso8601",
	}

	opts, err := cfg.ToOptions()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(opts.Development).To(BeTrue())
	g.Expect(opts.NewEncoder).ToNot(BeNil())
	g.Expect(opts.TimeEncoder).ToNot(BeNil())
}

func TestConfigToOptionsWithInvalidLevel(t *testing.T) {
	g := NewWithT(t)

	cfg := &configzap.Config{
		Level: "invalid-level",
	}

	_, err := cfg.ToOptions()
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid log level"))
}

func TestConfigToOptionsWithInvalidStacktraceLevel(t *testing.T) {
	g := NewWithT(t)

	cfg := &configzap.Config{
		StacktraceLevel: "invalid-level",
	}

	_, err := cfg.ToOptions()
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid stacktrace level"))
}

func TestConfigToOptionsWithInvalidTimeEncoding(t *testing.T) {
	g := NewWithT(t)

	cfg := &configzap.Config{
		TimeEncoding: "invalid-encoding",
	}

	_, err := cfg.ToOptions()
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid time encoding"))
}

func TestConfigWithAllLogLevels(t *testing.T) {
	tests := []struct {
		name  string
		level string
		valid bool
	}{
		{"debug level", "debug", true},
		{"info level", "info", true},
		{"warn level", "warn", true},
		{"error level", "error", true},
		{"dpanic level", "dpanic", true},
		{"panic level", "panic", true},
		{"fatal level", "fatal", true},
		{"invalid level", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cfg := &configzap.Config{
				Level: tt.level,
			}

			_, err := cfg.ToOptions()
			if tt.valid {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
			}
		})
	}
}

func TestConfigWithAllEncoders(t *testing.T) {
	tests := []struct {
		name    string
		encoder string
	}{
		{"json encoder", "json"},
		{"console encoder", "console"},
		{"empty encoder", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cfg := &configzap.Config{
				Encoder: tt.encoder,
			}

			opts, err := cfg.ToOptions()
			g.Expect(err).ToNot(HaveOccurred())

			if tt.encoder != "" {
				g.Expect(opts.NewEncoder).ToNot(BeNil())
			}
		})
	}
}

func TestConfigWithAllTimeEncodings(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		valid    bool
	}{
		{"epoch encoding", "epoch", true},
		{"millis encoding", "millis", true},
		{"nano encoding", "nano", true},
		{"iso8601 encoding", "iso8601", true},
		{"rfc3339 encoding", "rfc3339", true},
		{"rfc3339nano encoding", "rfc3339nano", true},
		{"invalid encoding", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cfg := &configzap.Config{
				TimeEncoding: tt.encoding,
			}

			opts, err := cfg.ToOptions()
			if tt.valid {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(opts.TimeEncoder).ToNot(BeNil())
			} else {
				g.Expect(err).To(HaveOccurred())
			}
		})
	}
}

func TestConfigBindFlags(t *testing.T) {
	g := NewWithT(t)

	cfg := &configzap.Config{
		Development:     false,
		Level:           "info",
		StacktraceLevel: "error",
		Encoder:         "json",
		TimeEncoding:    "epoch",
	}

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg.BindFlags(fs)

	// Verify flags are registered
	g.Expect(fs.Lookup("zap-devel")).ToNot(BeNil())
	g.Expect(fs.Lookup("zap-encoder")).ToNot(BeNil())
	g.Expect(fs.Lookup("zap-log-level")).ToNot(BeNil())
	g.Expect(fs.Lookup("zap-stacktrace-level")).ToNot(BeNil())
	g.Expect(fs.Lookup("zap-time-encoding")).ToNot(BeNil())
}

func TestConfigBindFlagsWithParse(t *testing.T) {
	g := NewWithT(t)

	cfg := &configzap.Config{
		Development: false,
		Level:       "info",
	}

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg.BindFlags(fs)

	err := fs.Parse([]string{
		"--zap-devel=true",
		"--zap-log-level=debug",
		"--zap-encoder=console",
		"--zap-stacktrace-level=warn",
		"--zap-time-encoding=iso8601",
	})
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchAllFields(Fields{
		"Development":     BeTrue(),
		"Level":           Equal("debug"),
		"StacktraceLevel": Equal("warn"),
		"Encoder":         Equal("console"),
		"TimeEncoding":    Equal("iso8601"),
	})))
}

func TestConfigLevelParsing(t *testing.T) {
	g := NewWithT(t)

	cfg := &configzap.Config{
		Level: "info",
	}

	opts, err := cfg.ToOptions()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(opts.Level.Enabled(zapcore.InfoLevel)).To(BeTrue())
	g.Expect(opts.Level.Enabled(zapcore.DebugLevel)).To(BeFalse())
}

func TestConfigStacktraceLevelParsing(t *testing.T) {
	g := NewWithT(t)

	cfg := &configzap.Config{
		StacktraceLevel: "error",
	}

	opts, err := cfg.ToOptions()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(opts.StacktraceLevel.Enabled(zapcore.ErrorLevel)).To(BeTrue())
	g.Expect(opts.StacktraceLevel.Enabled(zapcore.InfoLevel)).To(BeFalse())
}
