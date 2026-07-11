package logger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// defaultFlags specifies changes to the default logger behavior.  It is set
// during package init and configured using the LOGFLAGS environment variable.
// New logger backends can override these default flags using WithFlags.
var defaultFlags uint32

// Flags to modify Backend's behavior.
const (
	// Llongfile modifies the logger output to include full path and line number
	// of the logging callsite, e.g. /a/b/c/main.go:123.
	Llongfile uint32 = 1 << iota

	// Lshortfile modifies the logger output to include filename and line number
	// of the logging callsite, e.g. main.go:123.  Overrides Llongfile.
	Lshortfile
)

// Read logger flags from the LOGFLAGS environment variable.  Multiple flags can
// be set at once, separated by commas.
func init() {
	for _, f := range strings.Split(os.Getenv("LOGFLAGS"), ",") {
		switch f {
		case "longfile":
			defaultFlags |= Llongfile
		case "shortfile":
			defaultFlags |= Lshortfile
		}
	}
}

// Level is the level at which a logger is configured.  All messages sent
// to a level which is below the current level are filtered.
type Level uint32

// Level constants.
const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelCritical
	LevelOff
)

// levelStrs defines the human-readable names for each logging level.
var levelStrs = [...]string{"TRC", "DBG", "INF", "WRN", "ERR", "CRT", "OFF"}

// LevelFromString returns a level based on the input string s.  If the input
// can't be interpreted as a valid log level, the info level and false is
// returned.
func LevelFromString(s string) (l Level, ok bool) {
	switch strings.ToLower(s) {
	case "trace", "trc":
		return LevelTrace, true
	case "debug", "dbg":
		return LevelDebug, true
	case "info", "inf":
		return LevelInfo, true
	case "warn", "wrn":
		return LevelWarn, true
	case "error", "err":
		return LevelError, true
	case "critical", "crt":
		return LevelCritical, true
	case "off":
		return LevelOff, true
	default:
		return LevelInfo, false
	}
}

// String returns the tag of the logger used in log messages, or "OFF" if
// the level will not produce any log output.
func (l Level) String() string {
	if l >= LevelOff {
		return "OFF"
	}
	return levelStrs[l]
}

// NewBackend creates a logger backend from a Writer.
func NewBackend(w io.Writer, opts ...BackendOption) *Backend {
	b := &Backend{w: w, flag: defaultFlags}
	for _, o := range opts {
		o(b)
	}
	return b
}

// Backend is a logging backend.  Subsystems created from the backend write to
// the backend's Writer.  Backend provides atomic writes to the Writer from all
// subsystems.
type Backend struct {
	w    io.Writer
	mu   sync.Mutex // ensures atomic writes
	flag uint32
}

// BackendOption is a function used to modify the behavior of a Backend.
type BackendOption func(b *Backend)

// WithFlags configures a Backend to use the specified flags rather than using
// the package's defaults as determined through the LOGFLAGS environment
// variable.
func WithFlags(flags uint32) BackendOption {
	return func(b *Backend) {
		b.flag = flags
	}
}

// print outputs a log message to the writer associated with the backend after
// creating a prefix for the given level and tag according to the formatHeader
// function and formatting the provided arguments using the default formatting
// rules.
func (b *Backend) print(lvl, tag string, args ...interface{}) {
	t := time.Now() // get as early as possible

	bytebuf := buffer()

	var file string
	var line int
	if b.flag&(Lshortfile|Llongfile) != 0 {
		file, line = callsite(b.flag)
	}

	formatHeader(bytebuf, t, lvl, tag, file, line)
	buf := bytes.NewBuffer(*bytebuf)
	fmt.Fprintln(buf, args...)
	*bytebuf = buf.Bytes()

	b.mu.Lock()
	b.w.Write(*bytebuf)
	b.mu.Unlock()

	recycleBuffer(bytebuf)
}

// printf outputs a log message to the writer associated with the backend after
// creating a prefix for the given level and tag according to the formatHeader
// function and formatting the provided arguments according to the given format
// specifier.
func (b *Backend) printf(lvl, tag string, format string, args ...interface{}) {
	t := time.Now() // get as early as possible

	bytebuf := buffer()

	var file string
	var line int
	if b.flag&(Lshortfile|Llongfile) != 0 {
		file, line = callsite(b.flag)
	}

	formatHeader(bytebuf, t, lvl, tag, file, line)
	buf := bytes.NewBuffer(*bytebuf)
	fmt.Fprintf(buf, format, args...)
	*bytebuf = append(buf.Bytes(), '\n')

	b.mu.Lock()
	b.w.Write(*bytebuf)
	b.mu.Unlock()

	recycleBuffer(bytebuf)
}

// Logger returns a new logger for a particular subsystem that writes to the
// Backend b.  A tag describes the subsystem and is included in all log
// messages.  The logger uses the info verbosity level by default.
func (b *Backend) Logger(subsystemTag string) Logger {
	return &slog{LevelInfo, subsystemTag, b}
}

// slog is a subsystem logger for a Backend.  Implements the Logger interface.
type slog struct {
	lvl Level // atomic
	tag string
	b   *Backend
}

// Trace formats message using the default formats for its operands, prepends
// the prefix as necessary, and writes to log with LevelTrace.
//
// This is part of the Logger interface implementation.
func (l *slog) Trace(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelTrace {
		l.b.print("TRC", l.tag, args...)
	}
}

// Tracef formats message according to format specifier, prepends the prefix as
// necessary, and writes to log with LevelTrace.
//
// This is part of the Logger interface implementation.
func (l *slog) Tracef(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelTrace {
		l.b.printf("TRC", l.tag, format, args...)
	}
}

// Debug formats message using the default formats for its operands, prepends
// the prefix as necessary, and writes to log with LevelDebug.
//
// This is part of the Logger interface implementation.
func (l *slog) Debug(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelDebug {
		l.b.print("DBG", l.tag, args...)
	}
}

// Debugf formats message according to format specifier, prepends the prefix as
// necessary, and writes to log with LevelDebug.
//
// This is part of the Logger interface implementation.
func (l *slog) Debugf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelDebug {
		l.b.printf("DBG", l.tag, format, args...)
	}
}

// Info formats message using the default formats for its operands, prepends
// the prefix as necessary, and writes to log with LevelInfo.
//
// This is part of the Logger interface implementation.
func (l *slog) Info(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelInfo {
		l.b.print("INF", l.tag, args...)
	}
}

// Infof formats message according to format specifier, prepends the prefix as
// necessary, and writes to log with LevelInfo.
//
// This is part of the Logger interface implementation.
func (l *slog) Infof(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelInfo {
		l.b.printf("INF", l.tag, format, args...)
	}
}

// Warn formats message using the default formats for its operands, prepends
// the prefix as necessary, and writes to log with LevelWarn.
//
// This is part of the Logger interface implementation.
func (l *slog) Warn(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelWarn {
		l.b.print("WRN", l.tag, args...)
	}
}

// Warnf formats message according to format specifier, prepends the prefix as
// necessary, and writes to log with LevelWarn.
//
// This is part of the Logger interface implementation.
func (l *slog) Warnf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelWarn {
		l.b.printf("WRN", l.tag, format, args...)
	}
}

// Error formats message using the default formats for its operands, prepends
// the prefix as necessary, and writes to log with LevelError.
//
// This is part of the Logger interface implementation.
func (l *slog) Error(args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelError {
		l.b.print("ERR", l.tag, args...)
	}
}

// Errorf formats message according to format specifier, prepends the prefix as
// necessary, and writes to log with LevelError.
//
// This is part of the Logger interface implementation.
func (l *slog) Errorf(format string, args ...interface{}) {
	lvl := l.Level()
	if lvl <= LevelError {
		l.b.printf("ERR", l.tag, format, args...)
	}
}

// Level returns the current logging level
//
// This is part of the Logger interface implementation.
func (l *slog) Level() Level {
	return Level(atomic.LoadUint32((*uint32)(&l.lvl)))
}

// SetLevel changes the logging level to the passed level.
//
// This is part of the Logger interface implementation.
func (l *slog) SetLevel(level Level) {
	atomic.StoreUint32((*uint32)(&l.lvl), uint32(level))
}

// Disabled is a Logger that will never output anything.
var Disabled Logger

func init() {
	Disabled = &slog{lvl: LevelOff, b: NewBackend(io.Discard)}
}
