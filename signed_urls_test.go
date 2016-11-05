package main_test

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	. "github.com/petergtz/bitsgo"
	"github.com/petergtz/pegomock"
	. "github.com/petergtz/pegomock"
)

func TestSuite(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	pegomock.RegisterMockFailHandler(func(message string, callerSkip ...int) { panic(message) })
	ginkgo.RunSpecs(t, "SignedUrls")
}

var _ = Describe("Signing URLs", func() {
	It("signs and verifies URLs", func() {
		delegateHandler := NewMockHandler()

		handler := &SignedUrlHandler{
			Secret:           "geheim",
			Delegate:         delegateHandler,
			DelegateEndpoint: "secondhost",
		}

		// signing
		responseWriter := NewMockResponseWriter()
		handler.Sign(responseWriter, &http.Request{URL: mustParse("/sign/my/path")})

		responseBody := responseWriter.VerifyWasCalledOnce().Write(AnyUint8Slice()).GetCapturedArguments()
		Expect(string(responseBody)).To(ContainSubstring("http://secondhost/signed/my/path?md5="))

		// verifying
		responseWriter = NewMockResponseWriter()
		handler.Decode(responseWriter, &http.Request{URL: mustParse(string(responseBody))})
		responseWriter.VerifyWasCalled(Never()).WriteHeader(AnyInt())

		rw, request := delegateHandler.VerifyWasCalledOnce().ServeHTTP(AnyResponseWriter(), AnyRequestPtr()).GetCapturedArguments()
		Expect(request.URL.Path).To(Equal("/my/path"))
		Expect(request.URL.Host).To(Equal("secondhost"))
		Expect(rw).To(Equal(responseWriter))
	})

	FIt("bla", func() {
		signer := NewSignedS3UrlHandler()
		responseWriter := NewMockResponseWriter()
		signer.Sign(responseWriter, &http.Request{URL: mustParse("/sign/my/path")})
		s := responseWriter.VerifyWasCalledOnce().Write(AnyUint8Slice()).GetCapturedArguments()
		fmt.Println(string(s))
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
