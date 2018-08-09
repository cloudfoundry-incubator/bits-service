package logger

import (
	"bytes"
	"log"
	"net/http"

	"go.uber.org/zap"
)

var Log = setUpDefaultLogger().Sugar()

func setUpDefaultLogger() *zap.Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.DisableStacktrace = true
	logger, e := cfg.Build()
	if e != nil {
		panic(e)
	}
	return logger
}

func SetLogger(logger *zap.Logger) {
	Log = logger.Sugar()
}

func From(r *http.Request) *zap.SugaredLogger {
	log := r.Context().Value("logger")
	if log != nil {
		return log.(*zap.SugaredLogger)
	} else {
		return Log
	}
}

// Copied from go.uber.org/zap/global.go and changed to use Error instead of Info:
func NewStdLog(l *zap.Logger) *log.Logger {
	const (
		_stdLogDefaultDepth = 2
		_loggerWriterDepth  = 1
	)
	return log.New(&loggerWriter{l.WithOptions(
		zap.AddCallerSkip(_stdLogDefaultDepth + _loggerWriterDepth),
	)}, "" /* prefix */, 0 /* flags */)
}

type loggerWriter struct{ logger *zap.Logger }

func (l *loggerWriter) Write(p []byte) (int, error) {
	p = bytes.TrimSpace(p)
	l.logger.Error(string(p))
	return len(p), nil
}
