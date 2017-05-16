package logger

import (
	"net/http"

	"go.uber.org/zap"
)

var Log = setUpDefaultLogger().Sugar()

func setUpDefaultLogger() *zap.Logger {
	logger, e := zap.NewDevelopment()
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
