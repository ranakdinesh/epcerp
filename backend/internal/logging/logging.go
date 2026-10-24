// Package logging defines the application-owned structured logging contract.
package logging

import "context"

type Level uint8

const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
)

type Field struct {
	Key   string
	Value any
}

func String(key, value string) Field          { return Field{Key: key, Value: value} }
func Int(key string, value int) Field         { return Field{Key: key, Value: value} }
func Int32(key string, value int32) Field     { return Field{Key: key, Value: value} }
func Int64(key string, value int64) Field     { return Field{Key: key, Value: value} }
func Float64(key string, value float64) Field { return Field{Key: key, Value: value} }
func Bool(key string, value bool) Field       { return Field{Key: key, Value: value} }

type Event interface {
	Str(string, string) Event
	Int(string, int) Event
	Int32(string, int32) Event
	Int64(string, int64) Event
	Float64(string, float64) Event
	Bool(string, bool) Event
	Msg(string)
}

type Logger interface {
	Trace() Event
	Debug() Event
	Info() Event
	Warn() Event
	Error() Event
	WithLevel(Level) Event
	With(...Field) Logger
}

type contextKey struct{}

func WithContext(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

func Ctx(ctx context.Context) Logger {
	if logger, ok := ctx.Value(contextKey{}).(Logger); ok && logger != nil {
		return logger
	}
	return discardLogger{}
}

type discardLogger struct{}
type discardEvent struct{}

func (discardLogger) Trace() Event                 { return discardEvent{} }
func (discardLogger) Debug() Event                 { return discardEvent{} }
func (discardLogger) Info() Event                  { return discardEvent{} }
func (discardLogger) Warn() Event                  { return discardEvent{} }
func (discardLogger) Error() Event                 { return discardEvent{} }
func (discardLogger) WithLevel(Level) Event        { return discardEvent{} }
func (discardLogger) With(...Field) Logger         { return discardLogger{} }
func (discardEvent) Str(string, string) Event      { return discardEvent{} }
func (discardEvent) Int(string, int) Event         { return discardEvent{} }
func (discardEvent) Int32(string, int32) Event     { return discardEvent{} }
func (discardEvent) Int64(string, int64) Event     { return discardEvent{} }
func (discardEvent) Float64(string, float64) Event { return discardEvent{} }
func (discardEvent) Bool(string, bool) Event       { return discardEvent{} }
func (discardEvent) Msg(string)                    {}
