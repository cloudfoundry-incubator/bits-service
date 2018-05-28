package middlewares

import (
	"net/http"
	"strconv"

	"github.com/urfave/negroni"

	"regexp"
	"strings"
	"time"
)

type MetricsService interface {
	SendTimingMetric(name string, duration time.Duration)
	SendGaugeMetric(name string, value int64)
	SendCounterMetric(name string, value int64)
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
	duration := time.Since(startTime)
	responseStatus := strconv.Itoa(negroniResponseWriter.Status())
	middleware.metricsService.SendTimingMetric(request.Method+"-"+resourceType+"-time", duration)
	middleware.metricsService.SendTimingMetric(request.Method+"-"+resourceType+"-"+responseStatus+"-time", duration)
	middleware.metricsService.SendGaugeMetric(request.Method+"-"+resourceType+"-size", int64(negroniResponseWriter.Size()))
	middleware.metricsService.SendGaugeMetric(request.Method+"-"+resourceType+"-request-size", int64(request.ContentLength))
	middleware.metricsService.SendCounterMetric("status-"+responseStatus, 1)
}

var resourceURLPathPattern = regexp.MustCompile(`^/(packages|droplets|app_stash|buildpacks|buildpack_cache)/`)

func ResourceTypeFrom(path string) string {
	resourceType := strings.Trim(resourceURLPathPattern.FindString(path), "/")
	if resourceType == "" {
		return "invalid"
	}
	return resourceType
}
