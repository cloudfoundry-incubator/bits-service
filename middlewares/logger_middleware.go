package middlewares

import (
	"net/http"

	"math/rand"
	"time"

	"context"

	"github.com/urfave/negroni"
	"go.uber.org/zap"
)

type ZapLoggerMiddleware struct {
	logger *zap.SugaredLogger
}

func NewZapLoggerMiddleware(logger *zap.SugaredLogger) *ZapLoggerMiddleware {
	return &ZapLoggerMiddleware{logger: logger}
}

func (middleware *ZapLoggerMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	startTime := time.Now()

	requestLogger := middleware.logger.With("request-id", rand.Int63())

	requestLogger.Infow(
		"HTTP Request started",
		"host", request.Host,
		"method", request.Method,
		"path", request.URL.RequestURI(),
	)

	negroniResponseWriter, ok := responseWriter.(negroni.ResponseWriter)
	if !ok {
		negroniResponseWriter = negroni.NewResponseWriter(responseWriter)
	}

	next(negroniResponseWriter, request.WithContext(context.WithValue(nil, "logger", requestLogger)))

	fields := []interface{}{
		"host", request.Host,
		"method", request.Method,
		"path", request.URL.RequestURI(),
		"status-code", negroniResponseWriter.Status(),
		"body-size", negroniResponseWriter.Size(),
		"duration", time.Since(startTime),
	}
	if negroniResponseWriter.Status() >= 300 && negroniResponseWriter.Status() < 400 {
		fields = append(fields, zap.String("Location", negroniResponseWriter.Header().Get("Location")))
	}
	requestLogger.Infow("HTTP Request completed", fields...)
}
