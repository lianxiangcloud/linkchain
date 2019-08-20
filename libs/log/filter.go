package log

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type level byte

const (
	levelTrace level = 1 << iota
	levelDebug
	levelInfo
	levelWarn
	levelError
	levelCrit

	dateTimeFormat = "2006-01-02 15:04:05"
	dumpPrefix     = "Dump"
)

type reporter struct {
	oneCloudURL  string
	udpWriter    io.Writer
	prefix       []byte
	commonFields []byte
}

var canReport = false
var report *reporter = nil

type Filter struct {
	next           Logger
	allowed        level            // XOR'd levels for default case
	allowedKeyvals map[keyval]level // When key-value match, use this level
}

type keyval struct {
	key   interface{}
	value interface{}
}

// NewFilter wraps next and implements filtering. See the commentary on the
// Option functions for a detailed description of how to configure levels. If
// no options are provided, all leveled log events created with Debug, Info or
// Error helper methods are squelched.
func NewFilter(next Logger, options ...Option) Logger {
	l := &Filter{
		next:           next,
		allowedKeyvals: make(map[keyval]level),
	}
	for _, option := range options {
		option(l)
	}
	if report == nil {
		report = &reporter{
			udpWriter: nil,
		}
	}
	return l
}

func (l *Filter) Printf(format string, params ...interface{}) {
	l.next.Info(fmt.Sprintf(format, params...))
}

func (l *Filter) Println(format string, params ...interface{}) {
	l.next.Info(fmt.Sprintf(format, params...))
}

func (l *Filter) Trace(msg string, ctx ...interface{}) {
	levelAllowed := l.allowed&levelTrace != 0
	if !levelAllowed {
		return
	}
	l.next.Trace(msg, ctx...)
}

func (l *Filter) Debug(msg string, ctx ...interface{}) {
	levelAllowed := l.allowed&levelDebug != 0
	if !levelAllowed {
		return
	}
	l.next.Debug(msg, ctx...)
}

func (l *Filter) Info(msg string, ctx ...interface{}) {
	levelAllowed := l.allowed&levelInfo != 0
	if !levelAllowed {
		return
	}
	l.next.Info(msg, ctx...)
}

func (l *Filter) Warn(msg string, ctx ...interface{}) {
	levelAllowed := l.allowed&levelWarn != 0
	if !levelAllowed {
		return
	}
	l.next.Warn(msg, ctx...)
}

func (l *Filter) Error(msg string, ctx ...interface{}) {
	levelAllowed := l.allowed&levelError != 0
	if !levelAllowed {
		return
	}
	l.next.Error(msg, ctx...)
}

func (l *Filter) Crit(msg string, ctx ...interface{}) {
	levelAllowed := l.allowed&levelCrit != 0
	if !levelAllowed {
		return
	}
	l.next.Crit(msg, ctx...)
}

func (l *Filter) Report(msg string, ctx ...interface{}) {
	if canReport && len(ctx) > 1 && ctx[0].(string) == "logID" {
		l.reportMsg(ctx...)
	}
	levelAllowed := l.allowed&levelInfo != 0
	if !levelAllowed {
		return
	}
	l.next.Info(msg, ctx...)
}

func (l *Filter) Dump(msg string, ctx ...interface{}) {
	levelAllowed := l.allowed&levelInfo != 0
	if !levelAllowed {
		return
	}
	bz := make([]byte, 0, 256)
	bz = append(bz, []byte("\x01")...)
	for i := 0; i < len(ctx); i = i + 2 {
		key, ok := ctx[i].(string)
		if !ok {
			return
		}
		if i < len(ctx)-2 {
			bz = append(bz, []byte(fmt.Sprintf("%s,", key))...)
		} else {
			bz = append(bz, []byte(fmt.Sprintf("%s\x01", key))...)
		}
	}
	for i := 1; i < len(ctx); i = i + 2 {
		bz = append(bz, []byte(fmt.Sprintf("%s\x01", formatLogfmtValue(ctx[i], false)))...)
	}
	l.next.Info(fmt.Sprintf("%s\x01%s", dumpPrefix, msg), "Data", string(bz))
}

func (l *Filter) GetHandler() Handler {
	return l.next.GetHandler()
}

func (l *Filter) SetHandler(h Handler) {
	l.next.SetHandler(h)
}

func (l *Filter) SetBaseInfo(addr string, prefix string, role string) error {
	if report == nil {
		return fmt.Errorf("nil report")
	}
	if len(addr) == 0 {
		return nil
	}
	if len(prefix) == 0 || len(role) == 0 {
		return fmt.Errorf("nil params is not allowed")
	}

	if strings.HasPrefix(addr, "http://") && strings.HasSuffix(addr, "linke?hpbc=") {
		report.oneCloudURL = addr
	} else {
		conn, err := net.Dial("udp", addr)
		if err != nil {
			return err
		}
		report.udpWriter = conn
	}
	report.prefix = []byte(prefix + "\x01")
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "UNKNOW-HOST"
	}
	report.commonFields = []byte(fmt.Sprintf("\x01\x01%s\x01%s\x01\x01", hostname, role))
	canReport = true
	return nil
}

func (l *Filter) reportMsg(ctx ...interface{}) {
	logID, err := strconv.ParseInt(fmt.Sprintf("%v", ctx[1]), 0, 0)
	if err != nil {
		return
	}
	msg := make([]byte, 0, 256)
	msg = append(msg, report.prefix...)
	msg = append(msg, []byte(time.Now().Format(dateTimeFormat))...)
	msg = append(msg, report.commonFields...)
	msg = append(msg, []byte(fmt.Sprintf("%d\x01", logID))...)
	for i := 2; i < len(ctx); i = i + 2 {
		key, ok := ctx[i].(string)
		if !ok {
			return
		}
		if i < len(ctx)-2 {
			msg = append(msg, []byte(fmt.Sprintf("%s,", key))...)
		} else {
			msg = append(msg, []byte(fmt.Sprintf("%s\x01", key))...)
		}
	}
	for i := 3; i < len(ctx); i = i + 2 {
		msg = append(msg, []byte(fmt.Sprintf("%s\x01", formatLogfmtValue(ctx[i], false)))...)
	}
	if report.udpWriter != nil {
		report.udpWriter.Write(msg)
		return
	}
	resp, err := http.Get(report.oneCloudURL + url.QueryEscape(string(msg)))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Post OneCloud failed %s %d", resp.Status, resp.StatusCode)
	}
}

// With implements Logger by constructing a new Filter with a ctx appended
// to the logger.
//
// If custom level was set for a keyval pair using one of the
// Allow*With methods, it is used as the logger's level.
//
// Examples:
//     logger = log.NewFilter(logger, log.AllowError(), log.AllowInfoWith("module", "crypto"))
//		 logger.With("module", "crypto").Info("Hello") # produces "I... Hello module=crypto"
//
//     logger = log.NewFilter(logger, log.AllowError(), log.AllowInfoWith("module", "crypto"), log.AllowNoneWith("user", "Sam"))
//		 logger.With("module", "crypto", "user", "Sam").Info("Hello") # returns nil
//
//     logger = log.NewFilter(logger, log.AllowError(), log.AllowInfoWith("module", "crypto"), log.AllowNoneWith("user", "Sam"))
//		 logger.With("user", "Sam").With("module", "crypto").Info("Hello") # produces "I... Hello module=crypto user=Sam"
func (l *Filter) With(ctx ...interface{}) Logger {
	for i := len(ctx) - 2; i >= 0; i -= 2 {
		for kv, allowed := range l.allowedKeyvals {
			if ctx[i] == kv.key && ctx[i+1] == kv.value {
				return &Filter{next: l.next.With(ctx...), allowed: allowed, allowedKeyvals: l.allowedKeyvals}
			}
		}
	}
	return &Filter{next: l.next.With(ctx...), allowed: l.allowed, allowedKeyvals: l.allowedKeyvals}
}

//--------------------------------------------------------------------------------

// Option sets a parameter for the Filter.
type Option func(*Filter)

// AllowLevel returns an option for the given level or error if no option exist
// for such level.
func AllowLevel(lvl string) (Option, error) {
	switch lvl {
	case "trace":
		return AllowAll(), nil
	case "debug":
		return AllowDebug(), nil
	case "info":
		return AllowInfo(), nil
	case "warn":
		return AllowWarn(), nil
	case "error":
		return AllowError(), nil
	case "crit":
		return AllowCrit(), nil
	case "none":
		return AllowNone(), nil
	default:
		return nil, fmt.Errorf("Expected either \"info\", \"debug\", \"error\" or \"none\" level, given %s", lvl)
	}
}

const (
	lvlBaseTrace = levelCrit | levelError | levelWarn | levelInfo | levelDebug | levelTrace
	lvlBaseDebug = levelCrit | levelError | levelWarn | levelInfo | levelDebug
	lvlBaseInfo  = levelCrit | levelError | levelWarn | levelInfo
	lvlBaseWarn  = levelCrit | levelError | levelWarn
	lvlBaseError = levelCrit | levelError
)

func AllowAll() Option {
	return allowed(lvlBaseTrace)
}

func AllowDebug() Option {
	return allowed(lvlBaseDebug)
}

func AllowInfo() Option {
	return allowed(lvlBaseInfo)
}

func AllowWarn() Option {
	return allowed(lvlBaseWarn)
}

func AllowError() Option {
	return allowed(lvlBaseError)
}

func AllowCrit() Option {
	return allowed(levelCrit)
}

func AllowNone() Option {
	return allowed(0)
}

func allowed(allowed level) Option {
	return func(l *Filter) { l.allowed = allowed }
}

func AllowTranceWith(key interface{}, value interface{}) Option {
	return func(l *Filter) { l.allowedKeyvals[keyval{key, value}] = lvlBaseTrace }
}

func AllowDebugWith(key interface{}, value interface{}) Option {
	return func(l *Filter) { l.allowedKeyvals[keyval{key, value}] = lvlBaseDebug }
}

func AllowInfoWith(key interface{}, value interface{}) Option {
	return func(l *Filter) { l.allowedKeyvals[keyval{key, value}] = lvlBaseInfo }
}

func AllowWarnWith(key interface{}, value interface{}) Option {
	return func(l *Filter) { l.allowedKeyvals[keyval{key, value}] = lvlBaseWarn }
}

func AllowErrorWith(key interface{}, value interface{}) Option {
	return func(l *Filter) { l.allowedKeyvals[keyval{key, value}] = lvlBaseError }
}

func AllowCritWith(key interface{}, value interface{}) Option {
	return func(l *Filter) { l.allowedKeyvals[keyval{key, value}] = levelCrit }
}

func AllowNoneWith(key interface{}, value interface{}) Option {
	return func(l *Filter) { l.allowedKeyvals[keyval{key, value}] = 0 }
}
