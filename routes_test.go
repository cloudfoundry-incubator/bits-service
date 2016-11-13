package main_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	. "github.com/petergtz/bitsgo"
	. "github.com/petergtz/pegomock"
)

var _ = Describe("routes", func() {

	var (
		blobstore      *MockBlobstore
		router         *mux.Router
		responseWriter *httptest.ResponseRecorder
	)

	Describe("/packages/{guid}", func() {

		BeforeEach(func() {
			blobstore = NewMockBlobstore()
			router = mux.NewRouter()
			responseWriter = httptest.NewRecorder()
			SetUpPackageRoutes(router, blobstore)
		})

		Context("Method GET", func() {
			It("returns StatusOK and an empty body when blobstore is an empty implementation", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", "/packages/theguid", nil))

				Expect(*responseWriter).To(haveStatusAndBody(
					Equal(http.StatusOK),
					BeEmpty()))
			})

			It("returns StatusNotFound when blobstore writes StatusNotFound", func() {
				When(func() { blobstore.Get(AnyString(), AnyResponseWriter()) }).
					Then(writeStatusCodeAndBody(http.StatusNotFound, ""))

				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", "/packages/theguid", nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
			})

			It("returns StatusOK and fills body with contents from file located at the paritioned path", func() {
				When(func() { blobstore.Get("/th/eg/theguid", responseWriter) }).
					Then(writeStatusCodeAndBody(http.StatusOK, "thecontent"))

				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", "/packages/theguid", nil))

				Expect(*responseWriter).To(haveStatusAndBody(
					Equal(http.StatusOK),
					Equal("thecontent")))
			})
		})

		Context("Method PUT", func() {
			It("returns StatusBadRequest when package form file field is missing in request body", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("PUT", "/packages/theguid", nil))

				Expect(responseWriter.Code).To(Equal(http.StatusBadRequest))
			})

			XIt("returns StatusOK and an empty body when blobstore is an empty implementation", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("PUT", "/packages/theguid", nil))

				Expect(*responseWriter).To(haveStatusAndBody(
					Equal(http.StatusOK),
					BeEmpty()))
			})
		})

	})
})

func haveStatusAndBody(statusCode types.GomegaMatcher, body types.GomegaMatcher) types.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"Code": statusCode,
		"Body": WithTransform(func(body *bytes.Buffer) string { return body.String() }, body),
	})
}

func writeStatusCodeAndBody(statusCode int, body string) func([]Param) ReturnValues {
	return func(params []Param) ReturnValues {
		for _, param := range params {
			if responseWriter, ok := param.(http.ResponseWriter); ok {
				responseWriter.WriteHeader(statusCode)
				responseWriter.Write([]byte(body))
				return nil
			}
		}
		panic("Unexpected: no ResponseWriter in parameter list.")
	}
}
