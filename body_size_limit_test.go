package bitsgo_test

import (
	. "github.com/cloudfoundry-incubator/bits-service"
	"github.com/cloudfoundry-incubator/bits-service/httputil"

	"net/http"
	"net/http/httptest"

	"strings"

	"io/ioutil"

	"io"
)

type testHandler struct {
	maxBodySize              uint64
	manipulatedContentLength int64
}

func (handler *testHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	defer GinkgoRecover()
	if handler.manipulatedContentLength != 0 {
		manipulateContentLength(request, handler.manipulatedContentLength)
	}
	if !HandleBodySizeLimits(responseWriter, request, handler.maxBodySize) {
		return
	}
	_, e := io.Copy(responseWriter, request.Body)
	Expect(e).NotTo(HaveOccurred())
}

func manipulateContentLength(request *http.Request, newContentLength int64) {
	request.ContentLength = newContentLength
}

var _ = Describe("HandleBodySizeLimits", func() {

	It("returns StatusRequestEntityTooLarge when body size exceeds maxBodySize", func() {
		server := httptest.NewServer(&testHandler{maxBodySize: 5})

		response, e := http.DefaultClient.Do(httputil.NewRequest("GET", server.URL, strings.NewReader("Hello world!")).Build())
		Expect(e).NotTo(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusRequestEntityTooLarge))

		server.Close()
	})

	It("does not check body sizes at all when maxBodySize is 0", func() {
		server := httptest.NewServer(&testHandler{maxBodySize: 0})

		response, e := http.DefaultClient.Do(httputil.NewRequest("GET", server.URL, strings.NewReader("Hello world!")).Build())
		Expect(e).NotTo(HaveOccurred())
		Expect(ioutil.ReadAll(response.Body)).To(MatchRegexp("^Hello world!$"))

		server.Close()
	})

	It("uses the full body when the body size does not exceed maxBodySize", func() {
		server := httptest.NewServer(&testHandler{maxBodySize: 20})

		response, e := http.DefaultClient.Do(httputil.NewRequest("GET", server.URL, strings.NewReader("Hello world!")).Build())
		Expect(e).NotTo(HaveOccurred())
		Expect(ioutil.ReadAll(response.Body)).To(MatchRegexp("^Hello world!$"))

		server.Close()
	})

	It("uses ContentLength as authoritative source for body sizes when ContentLength and body size differ", func() {
		server := httptest.NewServer(&testHandler{maxBodySize: 20, manipulatedContentLength: 5})

		response, e := http.DefaultClient.Do(httputil.NewRequest("GET", server.URL, strings.NewReader("Hello world!")).Build())
		Expect(e).NotTo(HaveOccurred())
		Expect(ioutil.ReadAll(response.Body)).To(MatchRegexp("^Hello$"))

		server.Close()
	})

})
