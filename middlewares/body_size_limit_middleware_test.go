package middlewares_test

import (
	"strings"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry-incubator/bits-service/middlewares"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/petergtz/pegomock"
)

func TestMiddleWares(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	pegomock.RegisterMockFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Middleware")
}

var _ = Describe("BodySizeLimitMiddleWare", func() {
	Context("Body is larger than limit", func() {
		It("Fails when body is bigger than limit", func() {
			responseWriter := httptest.NewRecorder()

			middlewares.NewBodySizeLimitMiddleware(10).ServeHTTP(
				responseWriter,
				httptest.NewRequest("PUT", "/some/path", strings.NewReader("This is longer than the limit")),
				func(responseWriter http.ResponseWriter, request *http.Request) {
					Fail("Unexpected call to the next handler")
				})

			Expect(responseWriter.Code).To(Equal(http.StatusRequestEntityTooLarge), responseWriter.Body.String())
		})

	})

	Context("Body is smaller than limit", func() {
		It("succeeds", func() {
			responseWriter := httptest.NewRecorder()
			downstreamHandlerResponse := http.StatusOK

			middlewares.NewBodySizeLimitMiddleware(10000).ServeHTTP(
				responseWriter,
				httptest.NewRequest("PUT", "/some/path", strings.NewReader("This is shorter than the limit")),
				func(responseWriter http.ResponseWriter, request *http.Request) {
					responseWriter.WriteHeader(downstreamHandlerResponse)
				})

			Expect(responseWriter.Code).To(Equal(downstreamHandlerResponse), responseWriter.Body.String())
		})
	})

	Context("Limit is 0", func() {
		It("succeeds, even though body is technically larger than limi", func() {
			responseWriter := httptest.NewRecorder()
			downstreamHandlerResponse := http.StatusOK

			middlewares.NewBodySizeLimitMiddleware(0).ServeHTTP(
				responseWriter,
				httptest.NewRequest("PUT", "/some/path", strings.NewReader("This is longer than the limit")),
				func(responseWriter http.ResponseWriter, request *http.Request) {
					responseWriter.WriteHeader(downstreamHandlerResponse)
				})

			Expect(responseWriter.Code).To(Equal(downstreamHandlerResponse), responseWriter.Body.String())
		})
	})
})
