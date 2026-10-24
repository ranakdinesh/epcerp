// Package infrastructure provides runtime infrastructure outside business layers.
package infrastructure

import (
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/rs/zerolog"
)

type timestampHook struct{}

func (timestampHook) Run(event *zerolog.Event, _ zerolog.Level, _ string) {
	event.Str("timestamp", time.Now().UTC().Format(time.RFC3339Nano))
}

type Logger struct{ core zerolog.Logger }
type logEvent struct{ core *zerolog.Event }

var _ logging.Logger = (*Logger)(nil)
var _ logging.Event = logEvent{}

// NewLogger creates the runtime JSON logger on stdout.
func NewLogger(level string) (*Logger, error) {
	return NewLoggerWithWriter(os.Stdout, level)
}

// NewLoggerWithWriter allows tests to capture logs without changing global log settings.
func NewLoggerWithWriter(output io.Writer, level string) (*Logger, error) {
	if strings.TrimSpace(level) == "" {
		level = "info"
	}
	parsed, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(level)))
	if err != nil || parsed < zerolog.TraceLevel || parsed > zerolog.ErrorLevel {
		return nil, errors.New("invalid log level")
	}
	core := zerolog.New(zerolog.SyncWriter(output)).Level(parsed).With().
		Str("service", "epcERP").Str("component", "api").Logger().Hook(timestampHook{})
	return &Logger{core: core}, nil
}

func (l *Logger) Trace() logging.Event { return logEvent{core: l.core.Trace()} }
func (l *Logger) Debug() logging.Event { return logEvent{core: l.core.Debug()} }
func (l *Logger) Info() logging.Event  { return logEvent{core: l.core.Info()} }
func (l *Logger) Warn() logging.Event  { return logEvent{core: l.core.Warn()} }
func (l *Logger) Error() logging.Event { return logEvent{core: l.core.Error()} }
func (l *Logger) WithLevel(level logging.Level) logging.Event {
	levels := [...]zerolog.Level{zerolog.TraceLevel, zerolog.DebugLevel, zerolog.InfoLevel, zerolog.WarnLevel, zerolog.ErrorLevel}
	if int(level) >= len(levels) {
		return logEvent{core: l.core.Error()}
	}
	return logEvent{core: l.core.WithLevel(levels[level])}
}
func (l *Logger) With(fields ...logging.Field) logging.Logger {
	context := l.core.With()
	for _, field := range fields {
		context = context.Interface(field.Key, field.Value)
	}
	child := context.Logger()
	return &Logger{core: child}
}

func (e logEvent) Str(key, value string) logging.Event         { e.core.Str(key, value); return e }
func (e logEvent) Int(key string, value int) logging.Event     { e.core.Int(key, value); return e }
func (e logEvent) Int32(key string, value int32) logging.Event { e.core.Int32(key, value); return e }
func (e logEvent) Int64(key string, value int64) logging.Event { e.core.Int64(key, value); return e }
func (e logEvent) Float64(key string, value float64) logging.Event {
	e.core.Float64(key, value)
	return e
}
func (e logEvent) Bool(key string, value bool) logging.Event { e.core.Bool(key, value); return e }
func (e logEvent) Msg(message string)                        { e.core.Msg(message) }
