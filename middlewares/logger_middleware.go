package middlewares

import (
	"net/http"

	"math/rand"
	"time"

	"github.com/gorilla/context"
	"github.com/uber-go/zap"
	"github.com/urfave/negroni"
)

type ZapLoggerMiddleware struct {
	logger zap.Logger
}

func NewZapLoggerMiddleware(logger zap.Logger) *ZapLoggerMiddleware {
	return &ZapLoggerMiddleware{logger: logger}
}

func (middleware *ZapLoggerMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	startTime := time.Now()

	requestLogger := middleware.logger.With(zap.Int64("request-id", rand.Int63()))
	context.Set(request, "logger", requestLogger)

	requestLogger.Info(
		"HTTP Request started",
		zap.String("host", request.Host),
		zap.String("method", request.Method),
		zap.String("path", request.URL.Path),
	)

	negroniResponseWriter, ok := responseWriter.(negroni.ResponseWriter)
	if !ok {
		negroniResponseWriter = negroni.NewResponseWriter(responseWriter)
	}

	next(responseWriter, request)

	fields := []zap.Field{
		zap.String("host", request.Host),
		zap.String("method", request.Method),
		zap.String("path", request.URL.Path),
		zap.Int("status-code", negroniResponseWriter.Status()),
		zap.Int("body-size", negroniResponseWriter.Size()),
		zap.Duration("duration", time.Since(startTime)),
	}
	if negroniResponseWriter.Status() >= 300 && negroniResponseWriter.Status() < 400 {
		fields = append(fields, zap.String("Location", negroniResponseWriter.Header().Get("Location")))
	}
	requestLogger.Info("HTTP Request completed", fields...)

	context.Clear(request)
}
