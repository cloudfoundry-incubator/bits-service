package middlewares

import (
	"net/http"

	"github.com/urfave/negroni"

	"regexp"
	"strings"
	"time"
)

type MetricsService interface {
	SendTimingMetric(name string, duration time.Duration)
	SendGaugeMetric(name string, value int64)
}

type MetricsMiddleware struct {
	metricsService MetricsService
}

func NewMetricsMiddleware(metricsService MetricsService) *MetricsMiddleware {
	return &MetricsMiddleware{metricsService: metricsService}
}

func (middleware *MetricsMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	startTime := time.Now()
	negroniResponseWriter, ok := responseWriter.(negroni.ResponseWriter)
	if !ok {
		negroniResponseWriter = negroni.NewResponseWriter(responseWriter)
	}

	next(negroniResponseWriter, request)

	resourceType := ResourceTypeFrom(request.URL.Path)
	middleware.metricsService.SendTimingMetric(resourceType+"-time", time.Since(startTime))
	middleware.metricsService.SendGaugeMetric(resourceType+"-size", int64(negroniResponseWriter.Size()))
}

var resourceURLPathPattern = regexp.MustCompile(`^/(\w+)/`)

func ResourceTypeFrom(path string) string {
	return strings.Trim(resourceURLPathPattern.FindString(path), "/")
}
