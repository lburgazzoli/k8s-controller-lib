package zap

import (
	"fmt"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Config holds zap logger configuration.
// It mirrors the flags exposed by controller-runtime's zap.Options.BindFlags.
type Config struct {
	// Development configures the logger for development mode.
	// When true: encoder=console, logLevel=Debug, stackTraceLevel=Warn
	// When false: encoder=json, logLevel=Info, stackTraceLevel=Error
	Development bool `mapstructure:"development"`

	// Level configures the verbosity of logging.
	// Can be one of: debug, info, error, panic
	// Or any integer value > 0 for custom debug levels
	Level string `mapstructure:"log_level"`

	// StacktraceLevel sets the level at and above which stacktraces are captured.
	// Can be one of: info, error, panic
	StacktraceLevel string `mapstructure:"stacktrace_level"`

	// Encoder sets the log encoding format.
	// Can be one of: json, console
	Encoder string `mapstructure:"encoder"`

	// TimeEncoding sets the time format in log entries.
	// Can be one of: epoch, millis, nano, iso8601, rfc3339, rfc3339nano
	TimeEncoding string `mapstructure:"time_encoding"`
}

// ToOptions converts the configuration to controller-runtime's zap.Options.
func (c *Config) ToOptions() (ctrlzap.Options, error) {
	opts := ctrlzap.Options{
		Development: c.Development,
	}

	// Parse log level
	if c.Level != "" {
		level, err := zapcore.ParseLevel(c.Level)
		if err != nil {
			return opts, fmt.Errorf("invalid log level %q: %w", c.Level, err)
		}
		opts.Level = level
	}

	// Parse stacktrace level
	if c.StacktraceLevel != "" {
		level, err := zapcore.ParseLevel(c.StacktraceLevel)
		if err != nil {
			return opts, fmt.Errorf("invalid stacktrace level %q: %w", c.StacktraceLevel, err)
		}
		opts.StacktraceLevel = level
	}

	// Set encoder function
	if c.Encoder == "json" {
		opts.NewEncoder = func(optsFunc ...ctrlzap.EncoderConfigOption) zapcore.Encoder {
			encoderConfig := zap.NewProductionEncoderConfig()
			for _, opt := range optsFunc {
				opt(&encoderConfig)
			}
			return zapcore.NewJSONEncoder(encoderConfig)
		}
	} else if c.Encoder == "console" {
		opts.NewEncoder = func(optsFunc ...ctrlzap.EncoderConfigOption) zapcore.Encoder {
			encoderConfig := zap.NewDevelopmentEncoderConfig()
			for _, opt := range optsFunc {
				opt(&encoderConfig)
			}
			return zapcore.NewConsoleEncoder(encoderConfig)
		}
	}

	// Set time encoding
	if c.TimeEncoding != "" {
		var timeEncoder zapcore.TimeEncoder
		switch c.TimeEncoding {
		case "epoch":
			timeEncoder = zapcore.EpochTimeEncoder
		case "millis":
			timeEncoder = zapcore.EpochMillisTimeEncoder
		case "nano":
			timeEncoder = zapcore.EpochNanosTimeEncoder
		case "iso8601":
			timeEncoder = zapcore.ISO8601TimeEncoder
		case "rfc3339":
			timeEncoder = zapcore.RFC3339TimeEncoder
		case "rfc3339nano":
			timeEncoder = zapcore.RFC3339NanoTimeEncoder
		default:
			return opts, fmt.Errorf("invalid time encoding %q", c.TimeEncoding)
		}
		opts.TimeEncoder = timeEncoder
	}

	return opts, nil
}

// BindFlags adds zap configuration flags to the given FlagSet.
// This mirrors exactly what controller-runtime's zap.Options.BindFlags does,
// but binds to our Config struct instead.
func (c *Config) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.Development, "zap-devel", c.Development,
		"Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). "+
			"Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)")

	fs.StringVar(&c.Encoder, "zap-encoder", c.Encoder,
		"Zap log encoding (one of 'json' or 'console')")

	fs.StringVar(&c.Level, "zap-log-level", c.Level,
		"Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', 'panic'"+
			"or any integer value > 0 which corresponds to custom debug levels of increasing verbosity")

	fs.StringVar(&c.StacktraceLevel, "zap-stacktrace-level", c.StacktraceLevel,
		"Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').")

	fs.StringVar(&c.TimeEncoding, "zap-time-encoding", c.TimeEncoding,
		"Zap time encoding (one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'). Defaults to 'epoch'.")
}

