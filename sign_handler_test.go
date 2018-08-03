package bitsgo_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/cloudfoundry-incubator/bits-service"
	"github.com/cloudfoundry-incubator/bits-service/httputil"
	. "github.com/petergtz/pegomock"
)

func AnyTime() (result time.Time) {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*time.Time)(nil)).Elem()))
	return
}

var _ = Describe("SignResourceHandler", func() {
	var (
		getSigner *MockResourceSigner
		putSigner *MockResourceSigner
		recorder  *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		getSigner = NewMockResourceSigner()
		putSigner = NewMockResourceSigner()
		recorder = httptest.NewRecorder()
	})

	It("Signs a GET URL", func() {
		When(getSigner.Sign(AnyString(), AnyString(), AnyTime())).ThenReturn("Some get signature")
		handler := bitsgo.NewSignResourceHandler(getSigner, putSigner)
		request := httputil.NewRequest("GET", "/foo", nil).Build()

		handler.Sign(recorder, request, map[string]string{"verb": "get", "resource": "bar"})
		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(Equal("Some get signature"))
	})

	Context("Invalid method", func() {
		It("Responds with error code", func() {
			handler := bitsgo.NewSignResourceHandler(getSigner, putSigner)
			request := httputil.NewRequest("", "/does", nil).Build()

			handler.Sign(recorder, request, map[string]string{"verb": "something invalid", "resource": "foobar"})
			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			Expect(recorder.Body.String()).To(Equal("Invalid verb: something invalid"))
		})
	})

	It("Signs a PUT URL", func() {
		When(putSigner.Sign(AnyString(), AnyString(), AnyTime())).ThenReturn("Some put signature")

		handler := bitsgo.NewSignResourceHandler(getSigner, putSigner)
		request := httputil.NewRequest("PUT", "/bar", nil).Build()

		handler.Sign(recorder, request, map[string]string{"verb": "put", "resource": "foobar"})
		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(Equal("Some put signature"))
	})
})
