package logger

import (
	"fmt"

	"github.com/mobazha/mobazha/pkg/logging"
)

// LogWithID adds nodeID to a single log entry, supporting different log levels
func LogWithID(log *logging.Logger, nodeID string, level logging.Level, args ...interface{}) {
	msg := fmt.Sprintf("[%s] %s", nodeID, fmt.Sprint(args...))
	if nodeID == "" {
		msg = fmt.Sprint(args...)
	}
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

func LogInfoWithID(log *logging.Logger, nodeID string, args ...interface{}) {
	LogWithID(log, nodeID, logging.INFO, args...)
}

func LogDebugWithID(log *logging.Logger, nodeID string, args ...interface{}) {
	LogWithID(log, nodeID, logging.DEBUG, args...)
}

func LogNoticeWithID(log *logging.Logger, nodeID string, args ...interface{}) {
	LogWithID(log, nodeID, logging.NOTICE, args...)
}

func LogWarningWithID(log *logging.Logger, nodeID string, args ...interface{}) {
	LogWithID(log, nodeID, logging.WARNING, args...)
}

func LogErrorWithID(log *logging.Logger, nodeID string, args ...interface{}) {
	LogWithID(log, nodeID, logging.ERROR, args...)
}

func LogCriticalWithID(log *logging.Logger, nodeID string, args ...interface{}) {
	LogWithID(log, nodeID, logging.CRITICAL, args...)
}

// LogWithIDf adds nodeID to a single formatted log entry, supporting different log levels
func LogWithIDf(log *logging.Logger, nodeID string, level logging.Level, format string, args ...interface{}) {
	LogWithID(log, nodeID, level, fmt.Sprintf(format, args...))
}

func LogInfoWithIDf(log *logging.Logger, nodeID string, format string, args ...interface{}) {
	LogWithIDf(log, nodeID, logging.INFO, format, args...)
}

func LogDebugWithIDf(log *logging.Logger, nodeID string, format string, args ...interface{}) {
	LogWithIDf(log, nodeID, logging.DEBUG, format, args...)
}

func LogNoticeWithIDf(log *logging.Logger, nodeID string, format string, args ...interface{}) {
	LogWithIDf(log, nodeID, logging.NOTICE, format, args...)
}

func LogWarningWithIDf(log *logging.Logger, nodeID string, format string, args ...interface{}) {
	LogWithIDf(log, nodeID, logging.WARNING, format, args...)
}

func LogErrorWithIDf(log *logging.Logger, nodeID string, format string, args ...interface{}) {
	LogWithIDf(log, nodeID, logging.ERROR, format, args...)
}

func LogCriticalWithIDf(log *logging.Logger, nodeID string, format string, args ...interface{}) {
	LogWithIDf(log, nodeID, logging.CRITICAL, format, args...)
}
