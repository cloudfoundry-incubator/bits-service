package basic_auth_middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo/basic_auth_middleware"
	. "github.com/petergtz/bitsgo/testutil"
	"github.com/urfave/negroni"
)

func TestBasicAuthMiddleWare(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Basic Auth Middleware")
}

var _ = Describe("BasicAuthMiddle", func() {

	var (
		server *httptest.Server
	)

	BeforeEach(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			rw.Write([]byte("Hello"))
		})

		server = httptest.NewServer(negroni.New(
			&basic_auth_middleware.BasicAuthMiddleware{
				Username: "the-username",
				Password: "the-password",
			},
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
	})

	It("returns status forbidden when basic auth is invalid", func() {
		request := newGetRequest(server.URL)
		request.SetBasicAuth("the-username", "wrong-password")

		response, e := http.DefaultClient.Do(request)
		Expect(e).NotTo(HaveOccurred())

		Expect(*response).To(HaveStatusCodeAndBody(
			Equal(http.StatusForbidden),
			BeEmpty()))
	})

	It("returns status forbidden when basic auth is not set", func() {
		request := newGetRequest(server.URL)

		response, e := http.DefaultClient.Do(request)
		Expect(e).NotTo(HaveOccurred())

		Expect(*response).To(HaveStatusCodeAndBody(
			Equal(http.StatusForbidden),
			BeEmpty()))
	})
})

func newGetRequest(url string) *http.Request {
	request, e := http.NewRequest("GET", url, nil)
	Expect(e).NotTo(HaveOccurred())
	return request
}
