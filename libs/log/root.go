package log

import (
	"os"
)

var (
	test                 = &logger{[]interface{}{}, new(swapHandler)}
	root                 = &logger{[]interface{}{}, new(swapHandler)}
	base          Logger = root
	StdoutHandler        = StreamHandler(os.Stdout, LogfmtFormat())
	StderrHandler        = StreamHandler(os.Stderr, LogfmtFormat())
)

func init() {
	test.SetHandler(StdoutHandler)
	root.SetHandler(StdoutHandler)
}

// With returns a new logger with the given context.
// With is a convenient alias for Root().With
func With(ctx ...interface{}) Logger {
	return base.With(ctx...)
}
func New(ctx ...interface{}) Logger {
	return base.With(ctx...)
}

// Test returns the test logger
func Test() Logger {
	return test
}
func TestingLogger() Logger {
	return test
}

// Root returns the root logger
func Root() Logger {
	return root
}

// The following functions bypass the exported logger methods (logger.Debug,
// etc.) to keep the call depth the same for all paths to logger.write so
// runtime.Caller(2) always refers to the call site in client code.

// Trace is a convenient alias for Root().Trace
func Trace(msg string, ctx ...interface{}) {
	base.Trace(msg, ctx...)
}

// Debug is a convenient alias for Root().Debug
func Debug(msg string, ctx ...interface{}) {
	base.Debug(msg, ctx...)
}

// Info is a convenient alias for Root().Info
func Info(msg string, ctx ...interface{}) {
	base.Info(msg, ctx...)
}

// Warn is a convenient alias for Root().Warn
func Warn(msg string, ctx ...interface{}) {
	base.Warn(msg, ctx...)
}

// Error is a convenient alias for Root().Error
func Error(msg string, ctx ...interface{}) {
	base.Error(msg, ctx...)
}

// Crit is a convenient alias for Root().Crit
func Crit(msg string, ctx ...interface{}) {
	base.Crit(msg, ctx...)
}

func Report(msg string, ctx ...interface{}) {
	base.Report(msg, ctx...)
}

func Dump(msg string, ctx ...interface{}) {
	base.Dump(msg, ctx...)
}

// Output is a convenient alias for write, allowing for the modification of
// the calldepth (number of stack frames to skip).
// calldepth influences the reported line number of the log message.
// A calldepth of zero reports the immediate caller of Output.
// Non-zero calldepth skips as many stack frames.
func Output(msg string, lvl Lvl, calldepth int, ctx ...interface{}) {
	root.write(msg, lvl, ctx, calldepth+skipLevel)
}
