package middlewares_test

import (
	"net/http"
	"net/http/httptest"

	"bytes"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo/middlewares"
)

var _ = Describe("MetricsMiddleWare", func() {
	It("writes the correct values into the metrics writer and passes the request to the next middleware", func() {
		metricsWriter := &bytes.Buffer{}
		responseWriter := httptest.NewRecorder()
		middlewares.NewMetricsMiddleware(metricsWriter).ServeHTTP(
			responseWriter,
			httptest.NewRequest("GET", "/some/path", nil),
			func(responseWriter http.ResponseWriter, request *http.Request) {
				time.Sleep(time.Second)
				responseWriter.Write([]byte("Hello"))
			})

		Expect(responseWriter.Code).To(Equal(http.StatusOK))
		Expect(responseWriter.Body.String()).To(Equal("Hello"))

		Expect(metricsWriter.String()).To(MatchRegexp("GET /some/path HTTP/1.1;1.[0-9]+;5"))
	})
})
