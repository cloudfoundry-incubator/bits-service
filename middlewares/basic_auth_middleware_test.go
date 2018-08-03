package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/cloudfoundry-incubator/bits-service/middlewares"
	. "github.com/cloudfoundry-incubator/bits-service/testutil"
	"github.com/petergtz/pegomock"
	. "github.com/petergtz/pegomock"
	"github.com/urfave/negroni"
)

func TestBasicAuthMiddleWare(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	pegomock.RegisterMockFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Basic Auth Middleware")
}

var _ = Describe("BasicAuthMiddle", func() {

	var (
		server     *httptest.Server
		middleware *middlewares.BasicAuthMiddleware
		mux        *http.ServeMux
	)

	BeforeEach(func() {
		mux = http.NewServeMux()
		mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			rw.Write([]byte("Hello"))
		})
		middleware = middlewares.NewBasicAuthMiddleWare(
			middlewares.Credential{
				Username: "the-username",
				Password: "the-password",
			},
			middlewares.Credential{
				Username: "another-username",
				Password: "another-password",
			},
		)
	})

	JustBeforeEach(func() {
		server = httptest.NewServer(negroni.New(
			middleware,
			negroni.Wrap(mux),
		))
	})

	AfterEach(func() {
		server.Close()
	})

	It("returns status OK and the body from the handler when basic auth is valid", func() {
		request := newGetRequest(server.URL)
		request.SetBasicAuth("the-username", "the-password")

		response, e := http.DefaultClient.Do(request)
		Expect(e).NotTo(HaveOccurred())

		Expect(*response).To(HaveStatusCodeAndBody(
			Equal(http.StatusOK),
			MatchRegexp("Hello")))

		request = newGetRequest(server.URL)
		request.SetBasicAuth("another-username", "another-password")

		response, e = http.DefaultClient.Do(request)
		Expect(e).NotTo(HaveOccurred())

		Expect(*response).To(HaveStatusCodeAndBody(
			Equal(http.StatusOK),
			MatchRegexp("Hello")))
	})

	It("returns status unauthorized when basic auth is invalid", func() {
		request := newGetRequest(server.URL)
		request.SetBasicAuth("the-username", "wrong-password")

		response, e := http.DefaultClient.Do(request)
		Expect(e).NotTo(HaveOccurred())

		Expect(*response).To(HaveStatusCodeAndBody(
			Equal(http.StatusUnauthorized),
			BeEmpty()))
	})

	Context("unauthorizedHandler is set", func() {
		It("uses the unauthorizedHandler", func() {
			mockHandler := NewMockHandler()
			middleware.WithUnauthorizedHandler(mockHandler)

			request := newGetRequest(server.URL)
			request.SetBasicAuth("the-username", "wrong-password")

			_, e := http.DefaultClient.Do(request)
			Expect(e).NotTo(HaveOccurred())

			mockHandler.VerifyWasCalledOnce().ServeHTTP(anyResponseWriter(), anyRequestPtr())
		})
	})

	It("returns status unauthorized when basic auth is not set", func() {
		request := newGetRequest(server.URL)

		response, e := http.DefaultClient.Do(request)
		Expect(e).NotTo(HaveOccurred())

		Expect(*response).To(HaveStatusCodeAndBody(
			Equal(http.StatusUnauthorized),
			BeEmpty()))
	})
})

func newGetRequest(url string) *http.Request {
	request, e := http.NewRequest("GET", url, nil)
	Expect(e).NotTo(HaveOccurred())
	return request
}

func anyResponseWriter() http.ResponseWriter {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()))
	return nil
}

func anyRequestPtr() *http.Request {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*http.Request)(nil))))
	return nil
}
