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

	requestId := rand.Int63()
	requestLogger := middleware.logger.With(
		"request-id", requestId,
		"vcap-request-id", request.Header.Get("X-Vcap-Request-Id"))

	requestLogger.Infow(
		"HTTP Request started",
		"host", request.Host,
		"method", request.Method,
		"path", request.URL.RequestURI(),
		"user-agent", request.UserAgent(),
	)

	negroniResponseWriter, ok := responseWriter.(negroni.ResponseWriter)
	if !ok {
		negroniResponseWriter = negroni.NewResponseWriter(responseWriter)
	}

	requestContext := request.Context()
	requestContext = context.WithValue(requestContext, "logger", requestLogger)
	requestContext = context.WithValue(requestContext, "vcap-request-id", request.Header.Get("X-Vcap-Request-Id"))
	requestContext = context.WithValue(requestContext, "request-id", requestId)

	next(negroniResponseWriter, request.WithContext(requestContext))

	fields := []interface{}{
		"host", request.Host,
		"method", request.Method,
		"path", request.URL.RequestURI(),
		"status-code", negroniResponseWriter.Status(),
		"body-size", negroniResponseWriter.Size(),
		"duration", time.Since(startTime),
		"user-agent", request.UserAgent(),
	}
	if negroniResponseWriter.Status() >= 300 && negroniResponseWriter.Status() < 400 {
		fields = append(fields, zap.String("Location", negroniResponseWriter.Header().Get("Location")))
	}
	requestLogger.Infow("HTTP Request completed", fields...)
}
