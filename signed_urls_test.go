package main_test

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo/httputil"
	"github.com/petergtz/bitsgo/pathsigner"
	"github.com/urfave/negroni"

	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/petergtz/bitsgo"
)

var _ = Describe("Signing URLs", func() {
	It("signs and verifies URLs", func() {
		signer := &pathsigner.PathSigner{"geheim"}
		handler := &SignLocalUrlHandler{
			Signer:           signer,
			DelegateEndpoint: "http://example.com",
		}

		// signing
		responseWriter := httptest.NewRecorder()
		handler.Sign(responseWriter, &http.Request{URL: httputil.MustParse("/sign/my/path")})

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

})
