package gateway

import (
	"log"
	"os"
	"strings"
)

// Usable log levels
const (
	LogLevelSuppress = iota - 1
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

// DefaultLogger is the default logger from which each child logger is derived
var DefaultLogger = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)

// ChildLogger creates a child logger with the specified prefix
func ChildLogger(parent *log.Logger, prefix string) *log.Logger {
	return log.New(parent.Writer(), parent.Prefix()+prefix+" ", parent.Flags())
}

func (s *Shard) log(level int, format string, args ...interface{}) {
	if level > s.opts.LogLevel {
		return
	}

	s.opts.Logger.Printf(format+"\n", args...)
}

func (s *Shard) logTrace(trace []string) {
	if LogLevelDebug > s.opts.LogLevel {
		return
	}

	s.opts.Logger.Printf("Trace: %s\n", strings.Join(trace, " -> "))
}

func (s *Manager) log(level int, format string, args ...interface{}) {
	if level > s.opts.LogLevel {
		return
	}

	s.opts.Logger.Printf(format+"\n", args...)
}
