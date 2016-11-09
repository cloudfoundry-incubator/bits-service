package main_test

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"net/http/httptest"

	"github.com/gorilla/mux"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	. "github.com/petergtz/bitsgo"
	"github.com/petergtz/pegomock"
	. "github.com/petergtz/pegomock"
	"github.com/urfave/negroni"
)

func TestSuite(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	pegomock.RegisterMockFailHandler(func(message string, callerSkip ...int) { panic(message) })
	ginkgo.RunSpecs(t, "SignedUrls")
}

var _ = Describe("Signing URLs", func() {
	XIt("signs and verifies URLs", func() {
		delegateHandler := NewMockHandler()

		handler := &SignedLocalUrlHandler{
			Secret:           "geheim",
			DelegateEndpoint: "http://secondhost",
		}

		// signing
		responseWriter := NewMockResponseWriter()
		handler.Sign(responseWriter, &http.Request{URL: mustParse("/sign/my/path")})

		responseBody := fmt.Sprintf("%s", responseWriter.VerifyWasCalledOnce().Write(AnyUint8Slice()).GetCapturedArguments())
		Expect(responseBody).To(ContainSubstring("http://secondhost/my/path?md5="))

		// verifying
		responseWriter = NewMockResponseWriter()
		mux.NewRouter().Path("/packages/{guid}").Methods("GET").Handler(negroni.New(
			&SignatureVerifier{Secret: "geheim"},
			negroni.Wrap(delegateHandler),
		)).Subrouter().ServeHTTP(responseWriter, &http.Request{URL: mustParse(responseBody)})
		responseWriter.VerifyWasCalled(Never()).WriteHeader(AnyInt())

		rw, request := delegateHandler.VerifyWasCalledOnce().ServeHTTP(AnyResponseWriter(), AnyRequestPtr()).GetCapturedArguments()
		Expect(request.URL.Path).To(Equal("/my/path"))
		Expect(request.URL.Host).To(Equal("secondhost"))
		Expect(rw).To(Equal(responseWriter))
	})

	It("Can create pre-signed URLs for S3", func() {
		signer := NewSignedS3UrlHandler()
		responseWriter := httptest.NewRecorder()

		signer.Sign(responseWriter, &http.Request{URL: mustParse("/sign/my/path")})

		Expect(responseWriter.Body.String()).To(SatisfyAll(
			ContainSubstring("https://mybucket.s3.amazonaws.com/my/path"),
			ContainSubstring("X-Amz-Algorithm="),
			ContainSubstring("X-Amz-Credential=MY-Key_ID"),
			ContainSubstring("X-Amz-Date="),
			ContainSubstring("X-Amz-Expires="),
			ContainSubstring("X-Amz-Signature="),
		))
	})
})

func mustParse(rawUrl string) *url.URL {
	u, e := url.ParseRequestURI(rawUrl)
	Expect(e).NotTo(HaveOccurred())
	return u
}

func AnyRequestPtr() *http.Request {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*http.Request)(nil))))
	return nil
}

func AnyResponseWriter() http.ResponseWriter {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()))
	return nil
}
