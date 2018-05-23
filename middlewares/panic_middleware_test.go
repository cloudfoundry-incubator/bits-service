package middlewares_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo/middlewares"
	"github.com/pkg/errors"
)

var _ = Describe("PanicMiddleWare", func() {
	Context("Handler panics", func() {
		It("catches a panic and responds with an Internal Server Error", func() {
			responseWriter := httptest.NewRecorder()

			(&middlewares.PanicMiddleware{}).ServeHTTP(
				responseWriter,
				httptest.NewRequest("GET", "http://example.com/some/request", nil),
				func(http.ResponseWriter, *http.Request) {
					panic(errors.New("Some unexpected error"))
				})

			Expect(responseWriter.Code).To(Equal(http.StatusInternalServerError))
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
