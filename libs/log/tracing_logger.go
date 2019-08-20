package log

import (
	"fmt"

	"github.com/pkg/errors"
)

// NewTracingLogger enables tracing by wrapping all errors (if they
// implement stackTracer interface) in tracedError.
//
// All errors returned by https://github.com/pkg/errors implement stackTracer
// interface.
//
// For debugging purposes only as it doubles the amount of allocations.
func NewTracingLogger(next Logger) Logger {
	return &tracingLogger{
		next: next,
	}
}

type stackTracer interface {
	error
	StackTrace() errors.StackTrace
}

type tracingLogger struct {
	next Logger
}

func (l *tracingLogger) Printf(format string, params ...interface{}) {
	l.next.Info(fmt.Sprintf(format, params...))
}

func (l *tracingLogger) Println(format string, params ...interface{}) {
	l.next.Info(fmt.Sprintf(format, params...))
}

func (l *tracingLogger) Trace(msg string, ctx ...interface{}) {
	l.next.Trace(msg, formatErrors(ctx)...)
}

func (l *tracingLogger) Info(msg string, ctx ...interface{}) {
	l.next.Info(msg, formatErrors(ctx)...)
}

func (l *tracingLogger) Debug(msg string, ctx ...interface{}) {
	l.next.Debug(msg, formatErrors(ctx)...)
}

func (l *tracingLogger) Warn(msg string, ctx ...interface{}) {
	l.next.Warn(msg, formatErrors(ctx)...)
}

func (l *tracingLogger) Error(msg string, ctx ...interface{}) {
	l.next.Error(msg, formatErrors(ctx)...)
}

func (l *tracingLogger) Crit(msg string, ctx ...interface{}) {
	l.next.Crit(msg, formatErrors(ctx)...)
}

func (l *tracingLogger) Report(msg string, ctx ...interface{}) {
	l.next.Report(msg, formatErrors(ctx)...)
}

func (l *tracingLogger) Dump(msg string, ctx ...interface{}) {
	l.next.Dump(msg, formatErrors(ctx)...)
}

func (l *tracingLogger) With(ctx ...interface{}) Logger {
	return &tracingLogger{next: l.next.With(formatErrors(ctx)...)}
}

func (l *tracingLogger) GetHandler() Handler {
	return l.next.GetHandler()
}

func (l *tracingLogger) SetHandler(h Handler) {
	l.next.SetHandler(h)
}

func formatErrors(ctx []interface{}) []interface{} {
	newCtx := make([]interface{}, len(ctx))
	copy(newCtx, ctx)
	for i := 0; i < len(newCtx)-1; i += 2 {
		if err, ok := newCtx[i+1].(stackTracer); ok {
			newCtx[i+1] = tracedError{err}
		}
	}
	return newCtx
}

// tracedError wraps a stackTracer and just makes the Error() result
// always return a full stack trace.
type tracedError struct {
	wrapped stackTracer
}

var _ stackTracer = tracedError{}

func (t tracedError) StackTrace() errors.StackTrace {
	return t.wrapped.StackTrace()
}

func (t tracedError) Cause() error {
	return t.wrapped
}

func (t tracedError) Error() string {
	return fmt.Sprintf("%+v", t.wrapped)
}
