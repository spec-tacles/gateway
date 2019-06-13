package gateway

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// Logger represents a simple logger
type Logger interface {
	Printf(string, ...interface{})
}

// Usable log levels
const (
	LogLevelSuppress = iota - 1
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

func _log(l Logger, skip int, format string, args ...interface{}) {
	pc, file, line, _ := runtime.Caller(skip)
	fns := strings.Split(runtime.FuncForPC(pc).Name(), ".")

	l.Printf("%s:%d:%s() %s\n", filepath.Base(file), line, fns[len(fns)-1], fmt.Sprintf(format, args...))
}

func (s *Shard) log(level int, format string, args ...interface{}) {
	if level > s.opts.LogLevel {
		return
	}

	_log(s.opts.Logger, 2, format, args...)
}

func (s *Shard) logTrace(trace []string) {
	if LogLevelDebug > s.opts.LogLevel {
		return
	}

	_log(s.opts.Logger, 2, "Trace: %s", strings.Join(trace, " -> "))
}

func (s *Manager) log(level int, format string, args ...interface{}) {
	if level > s.opts.LogLevel {
		return
	}

	_log(s.opts.Logger, 2, format, args...)
}
