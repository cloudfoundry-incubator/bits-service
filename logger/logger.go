package logger

import (
	"net/http"

	"github.com/uber-go/zap"
)

var Log = zap.New(zap.NewTextEncoder(), zap.DebugLevel, zap.AddCaller())

func SetLogger(logger zap.Logger) {
	Log = logger
}

func From(r *http.Request) zap.Logger {
	log := r.Context().Value("logger")
	if log != nil {
		return log.(zap.Logger)
	} else {
		return Log
	}
}
