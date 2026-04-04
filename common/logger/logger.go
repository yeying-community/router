package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"gopkg.in/natefinch/lumberjack.v2"
)

type loggerLevel string

const (
	loggerDEBUG loggerLevel = "DEBUG"
	loggerINFO  loggerLevel = "INFO"
	loggerWarn  loggerLevel = "WARN"
	loggerError loggerLevel = "ERROR"
	loggerFatal loggerLevel = "FATAL"
)

var setupLogOnce sync.Once
var setupApiLogOnce sync.Once
var apiWriter io.Writer
var setupRelayLogOnce sync.Once
var relayWriter io.Writer
var routerInfoWriter io.Writer
var routerErrorWriter io.Writer
var errorWriter io.Writer
var setupWorkDirOnce sync.Once
var workDir string

func newRotatingWriter(path string) io.Writer {
	maxSizeMB := config.LogRotateMaxSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = 100
	}
	maxBackups := config.LogRotateMaxBackups
	if maxBackups < 0 {
		maxBackups = 10
	}
	maxAgeDays := config.LogRotateMaxAgeDays
	if maxAgeDays < 0 {
		maxAgeDays = 14
	}
	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    maxSizeMB,
		MaxBackups: maxBackups,
		MaxAge:     maxAgeDays,
		Compress:   config.LogRotateCompress,
	}
}

func SetupLogger() {
	setupLogOnce.Do(func() {
		logDir := LogDir
		if logDir == "" {
			logDir = "./logs"
		}
		_ = os.MkdirAll(logDir, 0755)
		routerPath := filepath.Join(logDir, "router.log")
		routerWriter := newRotatingWriter(routerPath)
		routerInfoWriter = io.MultiWriter(os.Stdout, routerWriter)
		routerErrorWriter = io.MultiWriter(os.Stderr, routerWriter)
		errorPath := filepath.Join(logDir, "error.log")
		errorWriter = newRotatingWriter(errorPath)

		SetupApiLogger()
		SetupRelayLogger()
		if apiWriter != nil {
			gin.DefaultWriter = io.MultiWriter(os.Stdout, apiWriter)
		}
		ginErrorWriter := routerErrorWriter
		if ginErrorWriter != nil && errorWriter != nil {
			ginErrorWriter = io.MultiWriter(ginErrorWriter, errorWriter)
		}
		if ginErrorWriter != nil {
			gin.DefaultErrorWriter = ginErrorWriter
		}
	})
}

// SetupApiLogger initializes api.log writer.
func SetupApiLogger() {
	setupApiLogOnce.Do(func() {
		logDir := LogDir
		if logDir == "" {
			logDir = "./logs"
		}
		_ = os.MkdirAll(logDir, 0755)
		logPath := filepath.Join(logDir, "api.log")
		apiWriter = newRotatingWriter(logPath)
	})
}

// SetupRelayLogger initializes relay.log writer.
func SetupRelayLogger() {
	setupRelayLogOnce.Do(func() {
		logDir := LogDir
		if logDir == "" {
			logDir = "./logs"
		}
		_ = os.MkdirAll(logDir, 0755)
		logPath := filepath.Join(logDir, "relay.log")
		relayWriter = newRotatingWriter(logPath)
	})
}

func SysLog(s string) {
	logHelper(nil, loggerINFO, s)
}

func SysLogf(format string, a ...any) {
	logHelper(nil, loggerINFO, fmt.Sprintf(format, a...))
}

func SysWarn(s string) {
	logHelper(nil, loggerWarn, s)
}

func SysWarnf(format string, a ...any) {
	logHelper(nil, loggerWarn, fmt.Sprintf(format, a...))
}

func SysError(s string) {
	logHelper(nil, loggerError, s)
}

func SysErrorf(format string, a ...any) {
	logHelper(nil, loggerError, fmt.Sprintf(format, a...))
}

func Debug(ctx context.Context, msg string) {
	if !config.DebugEnabled {
		return
	}
	logHelper(ctx, loggerDEBUG, msg)
}

func Info(ctx context.Context, msg string) {
	logHelper(ctx, loggerINFO, msg)
}

func Warn(ctx context.Context, msg string) {
	logHelper(ctx, loggerWarn, msg)
}

func Error(ctx context.Context, msg string) {
	logHelper(ctx, loggerError, msg)
}

func Debugf(ctx context.Context, format string, a ...any) {
	if !config.DebugEnabled {
		return
	}
	logHelper(ctx, loggerDEBUG, fmt.Sprintf(format, a...))
}

func Infof(ctx context.Context, format string, a ...any) {
	logHelper(ctx, loggerINFO, fmt.Sprintf(format, a...))
}

func Warnf(ctx context.Context, format string, a ...any) {
	logHelper(ctx, loggerWarn, fmt.Sprintf(format, a...))
}

func Errorf(ctx context.Context, format string, a ...any) {
	logHelper(ctx, loggerError, fmt.Sprintf(format, a...))
}

func FatalLog(s string) {
	logHelper(nil, loggerFatal, s)
}

func FatalLogf(format string, a ...any) {
	logHelper(nil, loggerFatal, fmt.Sprintf(format, a...))
}

// Loginf writes detailed login/auth related logs to router.log.
func Loginf(ctx context.Context, format string, a ...any) {
	logHelper(ctx, loggerINFO, "[login] "+fmt.Sprintf(format, a...))
}

func LoginErrorf(ctx context.Context, format string, a ...any) {
	logHelper(ctx, loggerError, "[login] "+fmt.Sprintf(format, a...))
}

// ApiLogf writes per-request api logs to api.log
func ApiLogf(ctx context.Context, level loggerLevel, format string, a ...any) {
	apiLogHelper(ctx, level, fmt.Sprintf(format, a...))
}

func RelayInfof(ctx context.Context, format string, a ...any) {
	relayLogHelper(ctx, loggerINFO, fmt.Sprintf(format, a...))
}

func RelayWarnf(ctx context.Context, format string, a ...any) {
	relayLogHelper(ctx, loggerWarn, fmt.Sprintf(format, a...))
}

func RelayErrorf(ctx context.Context, format string, a ...any) {
	relayLogHelper(ctx, loggerError, fmt.Sprintf(format, a...))
}

func isErrorLogLevel(level loggerLevel) bool {
	return level == loggerError || level == loggerFatal
}

func formatLogLine(now time.Time, level loggerLevel, traceID string, lineInfo string, funcName string, msg string) string {
	return fmt.Sprintf("%v [%s]%s%s %s%s \n", now.Format("2006/01/02 - 15:04:05"), level, traceID, lineInfo, funcName, msg)
}

func logHelper(ctx context.Context, level loggerLevel, msg string) {
	SetupLogger()
	writer := routerErrorWriter
	if level == loggerINFO {
		writer = routerInfoWriter
	}
	if writer == nil {
		writer = gin.DefaultErrorWriter
		if level == loggerINFO {
			writer = gin.DefaultWriter
		}
	}
	var traceID string
	if ctx != nil {
		rawTraceID := helper.GetTraceID(ctx)
		if rawTraceID != "" {
			traceID = fmt.Sprintf(" %s", rawTraceID)
		}
	}
	lineInfo, funcName := getLineInfo()
	now := time.Now()
	line := formatLogLine(now, level, traceID, lineInfo, funcName, msg)
	_, _ = io.WriteString(writer, line)
	if isErrorLogLevel(level) && errorWriter != nil {
		_, _ = io.WriteString(errorWriter, line)
	}
	if level == loggerFatal {
		os.Exit(1)
	}
}

// apiLogHelper mirrors logHelper but targets api.log
func apiLogHelper(ctx context.Context, level loggerLevel, msg string) {
	SetupLogger()
	writer := apiWriter
	if writer == nil {
		logHelper(ctx, level, "[api] "+msg)
		return
	}
	var traceID string
	if ctx != nil {
		rawTraceID := helper.GetTraceID(ctx)
		if rawTraceID != "" {
			traceID = fmt.Sprintf(" %s", rawTraceID)
		}
	}
	lineInfo, funcName := getLineInfo()
	now := time.Now()
	line := formatLogLine(now, level, traceID, lineInfo, funcName, "[api] "+msg)
	_, _ = io.WriteString(writer, line)
	if isErrorLogLevel(level) && errorWriter != nil {
		_, _ = io.WriteString(errorWriter, line)
	}
}

func relayLogHelper(ctx context.Context, level loggerLevel, msg string) {
	SetupLogger()
	writer := relayWriter
	if writer == nil {
		logHelper(ctx, level, "[relay] "+msg)
		return
	}
	var traceID string
	if ctx != nil {
		rawTraceID := helper.GetTraceID(ctx)
		if rawTraceID != "" {
			traceID = fmt.Sprintf(" %s", rawTraceID)
		}
	}
	lineInfo, funcName := getLineInfo()
	now := time.Now()
	line := formatLogLine(now, level, traceID, lineInfo, funcName, "[relay] "+msg)
	_, _ = io.WriteString(writer, line)
	if isErrorLogLevel(level) && errorWriter != nil {
		_, _ = io.WriteString(errorWriter, line)
	}
}

func getWorkDir() string {
	setupWorkDirOnce.Do(func() {
		wd, err := os.Getwd()
		if err == nil {
			workDir = filepath.Clean(wd)
		}
	})
	return workDir
}

func makeRelativePath(file string) string {
	cleanFile := filepath.Clean(file)
	root := getWorkDir()
	if root != "" {
		if rel, err := filepath.Rel(root, cleanFile); err == nil {
			rel = filepath.ToSlash(rel)
			if rel != "" && rel != "." && !strings.HasPrefix(rel, "../") {
				return rel
			}
		}
	}
	normalized := filepath.ToSlash(cleanFile)
	for _, marker := range []string{"/internal/", "/common/", "/cmd/", "/docs/", "/scripts/", "/web/"} {
		if idx := strings.Index(normalized, marker); idx >= 0 {
			return normalized[idx+1:]
		}
	}
	return normalized
}

func getLineInfo() (string, string) {
	funcName := "[unknown] "
	pc, file, line, ok := runtime.Caller(3)
	if ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			parts := strings.Split(fn.Name(), ".")
			funcName = "[" + parts[len(parts)-1] + "] "
		}
	} else {
		file = "unknown"
		line = 0
	}
	return fmt.Sprintf(" %s:%d", makeRelativePath(file), line), funcName
}
