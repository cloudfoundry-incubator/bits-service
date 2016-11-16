package main_test

import (
	"io"
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

var _ = Describe("PathSigner", func() {
	It("Can sign a path and validate its signature", func() {
		signer := &PathSigner{"thesecret"}

		signedPath := signer.Sign("/some/path")

		Expect(signer.SignatureValid(mustParse(signedPath))).To(BeTrue())
	})
})

var _ = Describe("Signing URLs", func() {
	It("signs and verifies URLs", func() {
		signer := &PathSigner{"geheim"}
		handler := &SignLocalUrlHandler{
			Signer:           signer,
			DelegateEndpoint: "http://example.com",
		}

		// signing
		responseWriter := httptest.NewRecorder()
		handler.Sign(responseWriter, &http.Request{URL: mustParse("/sign/my/path")})

		responseBody := responseWriter.Body.String()

		Expect(responseBody).To(ContainSubstring("http://example.com/my/path?md5="))

		// verifying
		responseWriter = httptest.NewRecorder()
		delegateHandler := NewMockHandler()

		r := mux.NewRouter()
		r.Path("/my/path").Methods("GET").Handler(negroni.New(
			&SignatureVerificationMiddleware{signer},
			negroni.Wrap(delegateHandler),
		))
		r.ServeHTTP(responseWriter, httptest.NewRequest("GET", responseBody, nil))

		Expect(responseWriter.Code).To(Equal(http.StatusOK))
	})

	It("Can create pre-signed URLs for S3", func() {
		signer := NewSignS3UrlHandler()
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

func AnyReadSeeker() io.ReadSeeker {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*io.ReadSeeker)(nil)).Elem()))
	return nil
}
