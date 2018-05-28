package middlewares_test

import (
	http "net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo/middlewares"
	. "github.com/petergtz/bitsgo/middlewares/matchers"
	. "github.com/petergtz/pegomock"
)

var _ = Describe("MetricsMiddleWare", func() {
	It("can properly extract resource types from URL path", func() {
		Expect(middlewares.ResourceTypeFrom("/packages/123456")).To(Equal("packages"))
		Expect(middlewares.ResourceTypeFrom("/packages/123456/789")).To(Equal("packages"))
		Expect(middlewares.ResourceTypeFrom("/droplets/123456/789")).To(Equal("droplets"))
		Expect(middlewares.ResourceTypeFrom("/app_stash/some-sha")).To(Equal("app_stash"))
		Expect(middlewares.ResourceTypeFrom("/buildpacks/123456/789")).To(Equal("buildpacks"))
		Expect(middlewares.ResourceTypeFrom("/buildpack_cache/entries/123456/789")).To(Equal("buildpack_cache"))
		Expect(middlewares.ResourceTypeFrom("/")).To(Equal("invalid"))
		Expect(middlewares.ResourceTypeFrom("/something/else")).To(Equal("invalid"))
	})

	It("sends all required metrics", func() {
		metricsService := NewMockMetricsService()
		middleware := middlewares.NewMetricsMiddleware(metricsService)
		req, e := http.NewRequest("GET", "http://example.com/packages/someguid", nil)
		Expect(e).NotTo(HaveOccurred())

		middleware.ServeHTTP(httptest.NewRecorder(), req, func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusForbidden)
		})

		metricsService.VerifyWasCalledOnce().SendCounterMetric("status-403", 1)
		metricsService.VerifyWasCalledOnce().SendGaugeMetric("GET-packages-size", 0)
		metricsService.VerifyWasCalledOnce().SendGaugeMetric("GET-packages-request-size", 0)
		metricsService.VerifyWasCalledOnce().SendTimingMetric(EqString("GET-packages-time"), AnyTimeDuration())
		metricsService.VerifyWasCalledOnce().SendTimingMetric(EqString("GET-packages-403-time"), AnyTimeDuration())
	})
})
