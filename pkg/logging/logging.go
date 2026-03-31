package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
)

// Level defines supported logging levels.
type Level int

const (
	DEBUG Level = iota
	INFO
	NOTICE
	WARNING
	ERROR
	CRITICAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case NOTICE:
		return "notice"
	case WARNING:
		return "warning"
	case ERROR:
		return "error"
	case CRITICAL:
		return "critical"
	default:
		return "info"
	}
}

func (l Level) toSlog() slog.Level {
	switch l {
	case DEBUG:
		return slog.LevelDebug
	case INFO:
		return slog.LevelInfo
	case NOTICE:
		return slog.LevelInfo + 1
	case WARNING:
		return slog.LevelWarn
	case ERROR:
		return slog.LevelError
	case CRITICAL:
		return slog.LevelError + 4
	default:
		return slog.LevelInfo
	}
}

// ParseLevel parses text level into Level.
func ParseLevel(raw string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return DEBUG, nil
	case "info":
		return INFO, nil
	case "notice":
		return NOTICE, nil
	case "warn", "warning":
		return WARNING, nil
	case "err", "error":
		return ERROR, nil
	case "critical", "fatal":
		return CRITICAL, nil
	default:
		return INFO, fmt.Errorf("invalid log level: %s", raw)
	}
}

// Format controls slog output mode.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Config controls global logger behavior.
type Config struct {
	Level     Level
	Format    Format
	AddSource bool
	Writers   []io.Writer
}

type runtimeConfig struct {
	cfg  Config
	base *slog.Logger
}

var current atomic.Pointer[runtimeConfig]

func init() {
	Configure(Config{
		Level:   INFO,
		Format:  FormatText,
		Writers: []io.Writer{os.Stdout},
	})
}

func normalizeConfig(cfg Config) Config {
	if cfg.Format == "" {
		cfg.Format = FormatText
	}
	if len(cfg.Writers) == 0 {
		cfg.Writers = []io.Writer{os.Stdout}
	}
	writers := make([]io.Writer, 0, len(cfg.Writers))
	for _, w := range cfg.Writers {
		if w != nil {
			writers = append(writers, w)
		}
	}
	if len(writers) == 0 {
		writers = []io.Writer{os.Stdout}
	}
	cfg.Writers = writers
	return cfg
}

func buildLogger(cfg Config) *slog.Logger {
	writer := io.MultiWriter(cfg.Writers...)
	handlerOptions := &slog.HandlerOptions{
		AddSource: cfg.AddSource,
		Level:     cfg.Level.toSlog(),
	}

	var handler slog.Handler
	if cfg.Format == FormatJSON {
		handler = slog.NewJSONHandler(writer, handlerOptions)
	} else {
		handler = slog.NewTextHandler(writer, handlerOptions)
	}
	return slog.New(handler)
}

// Configure updates global logger runtime settings.
func Configure(cfg Config) {
	cfg = normalizeConfig(cfg)
	current.Store(&runtimeConfig{
		cfg:  cfg,
		base: buildLogger(cfg),
	})
}

// CurrentConfig returns a snapshot of global config.
func CurrentConfig() Config {
	rc := current.Load()
	if rc == nil {
		return Config{Level: INFO, Format: FormatText, Writers: []io.Writer{os.Stdout}}
	}
	cfg := rc.cfg
	writers := make([]io.Writer, len(cfg.Writers))
	copy(writers, cfg.Writers)
	cfg.Writers = writers
	return cfg
}

// SetLevel updates global log level while preserving other config values.
func SetLevel(level Level) {
	cfg := CurrentConfig()
	cfg.Level = level
	Configure(cfg)
}

func baseLogger() *slog.Logger {
	rc := current.Load()
	if rc != nil && rc.base != nil {
		return rc.base
	}
	return slog.Default()
}

// Logger wraps slog.Logger with compatibility helpers.
type Logger struct {
	module string
	attrs  []any
}

// MustGetLogger creates module logger.
func MustGetLogger(module string) *Logger {
	return &Logger{module: module}
}

// GetLogger creates module logger and returns an error only for invalid module names.
func GetLogger(module string) (*Logger, error) {
	if strings.TrimSpace(module) == "" {
		return nil, fmt.Errorf("logger module cannot be empty")
	}
	return MustGetLogger(module), nil
}

// With adds structured key/value attrs to logger.
func (l *Logger) With(args ...any) *Logger {
	if len(args) == 0 {
		return l
	}
	clone := &Logger{module: l.module}
	clone.attrs = append(clone.attrs, l.attrs...)
	clone.attrs = append(clone.attrs, args...)
	return clone
}

func (l *Logger) slog() *slog.Logger {
	logger := baseLogger()
	if l.module != "" {
		logger = logger.With("module", l.module)
	}
	if len(l.attrs) > 0 {
		logger = logger.With(l.attrs...)
	}
	return logger
}

func (l *Logger) log(level Level, msg string) {
	l.slog().Log(context.Background(), level.toSlog(), msg)
}

func (l *Logger) Debug(args ...interface{})    { l.log(DEBUG, fmt.Sprint(args...)) }
func (l *Logger) Info(args ...interface{})     { l.log(INFO, fmt.Sprint(args...)) }
func (l *Logger) Notice(args ...interface{})   { l.log(NOTICE, fmt.Sprint(args...)) }
func (l *Logger) Warn(args ...interface{})     { l.log(WARNING, fmt.Sprint(args...)) }
func (l *Logger) Warning(args ...interface{})  { l.log(WARNING, fmt.Sprint(args...)) }
func (l *Logger) Error(args ...interface{})    { l.log(ERROR, fmt.Sprint(args...)) }
func (l *Logger) Critical(args ...interface{}) { l.log(CRITICAL, fmt.Sprint(args...)) }
func (l *Logger) Fatal(args ...interface{}) {
	l.log(CRITICAL, fmt.Sprint(args...))
	os.Exit(1)
}
func (l *Logger) Panic(args ...interface{}) {
	msg := fmt.Sprint(args...)
	l.log(CRITICAL, msg)
	panic(msg)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, args...))
}
func (l *Logger) Infof(format string, args ...interface{}) { l.log(INFO, fmt.Sprintf(format, args...)) }
func (l *Logger) Noticef(format string, args ...interface{}) {
	l.log(NOTICE, fmt.Sprintf(format, args...))
}
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WARNING, fmt.Sprintf(format, args...))
}
func (l *Logger) Warningf(format string, args ...interface{}) {
	l.log(WARNING, fmt.Sprintf(format, args...))
}
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, args...))
}
func (l *Logger) Criticalf(format string, args ...interface{}) {
	l.log(CRITICAL, fmt.Sprintf(format, args...))
}
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(CRITICAL, fmt.Sprintf(format, args...))
	os.Exit(1)
}
func (l *Logger) Panicf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.log(CRITICAL, msg)
	panic(msg)
}
