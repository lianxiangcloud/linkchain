package log

type nopLogger struct{ h *swapHandler }

// Interface assertions
var _ Logger = (*nopLogger)(nil)

// NewNopLogger returns a logger that doesn't do anything.
func NewNopLogger() Logger { return &nopLogger{} }

func (l *nopLogger) With(...interface{}) Logger {
	return l
}

func (nopLogger) Printf(format string, params ...interface{}) {}
func (nopLogger) Println(format string, params ...interface{}) {}
func (nopLogger) Trace(string, ...interface{})  {}
func (nopLogger) Debug(string, ...interface{})  {}
func (nopLogger) Info(string, ...interface{})   {}
func (nopLogger) Warn(string, ...interface{})   {}
func (nopLogger) Error(string, ...interface{})  {}
func (nopLogger) Crit(string, ...interface{})   {}
func (nopLogger) Report(string, ...interface{}) {}
func (nopLogger) Dump(string, ...interface{})   {}

func (l *nopLogger) GetHandler() Handler {
	return l.h.Get()
}

func (l *nopLogger) SetHandler(h Handler) {
	l.h.Swap(h)
}
