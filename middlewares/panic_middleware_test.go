package middlewares_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry-incubator/bits-service/middlewares"
	"github.com/cloudfoundry-incubator/bits-service/util"
	"github.com/pkg/errors"
)

var _ = Describe("PanicMiddleWare", func() {
	Context("Handler panics", func() {
		It("catches a panic and responds with an Internal Server Error", func() {
			responseWriter := httptest.NewRecorder()

			(&middlewares.PanicMiddleware{}).ServeHTTP(
				responseWriter,
				util.RequestWithContextValues(
					httptest.NewRequest("GET", "http://example.com/some/request", nil),
					"request-id", int64(123456),
					"vcap-request-id", "123456-7890-1234"),
				func(http.ResponseWriter, *http.Request) {
					panic(errors.New("Some unexpected error"))
				})

			Expect(responseWriter.Code).To(Equal(http.StatusInternalServerError))
			Expect(responseWriter.Body.String()).To(MatchJSON(`{
				"service": "Bits-Service",
				"error": "Internal Server Error",
				"request-id": 123456,
				"vcap-request-id":"123456-7890-1234"
			}`))
		})
	})

	Context("Handler succeeds", func() {
		It("responds with handler's response", func() {
			responseWriter := httptest.NewRecorder()

			(&middlewares.PanicMiddleware{}).ServeHTTP(
				responseWriter,
				httptest.NewRequest("GET", "http://example.com/some/request", nil),
				func(rw http.ResponseWriter, r *http.Request) {
					rw.WriteHeader(http.StatusOK)
				})

			Expect(responseWriter.Code).To(Equal(http.StatusOK))
		})
	})
})
