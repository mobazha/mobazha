package logger

import (
	"fmt"

	"github.com/op/go-logging"
)

// LogWithID adds nodeID to a single log entry, supporting different log levels
func LogWithID(log *logging.Logger, nodeID string, level logging.Level, args ...interface{}) {
	msg := fmt.Sprintf("[%s] %s", nodeID, fmt.Sprint(args...))
	switch level {
	case logging.DEBUG:
		log.Debug(msg)
	case logging.INFO:
		log.Info(msg)
	case logging.NOTICE:
		log.Notice(msg)
	case logging.WARNING:
		log.Warning(msg)
	case logging.ERROR:
		log.Error(msg)
	case logging.CRITICAL:
		log.Critical(msg)
	}
}

// LogWithIDf adds nodeID to a single formatted log entry, supporting different log levels
func LogWithIDf(log *logging.Logger, nodeID string, level logging.Level, format string, args ...interface{}) {
	LogWithID(log, nodeID, level, fmt.Sprintf(format, args...))
}
