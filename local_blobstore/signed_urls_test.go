package local_blobstore_test

import (
	"net/http"
	"testing"

	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo/pathsigner"
	"github.com/urfave/negroni"

	"net/http/httptest"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	. "github.com/petergtz/bitsgo/local_blobstore"
)

func TestLocalBlobstore(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "LocalBlobstore")
}

var _ = Describe("Signing URLs", func() {
	It("signs and verifies URLs", func() {
		signer := &pathsigner.PathSigner{"geheim"}
		handler := &LocalResourceSigner{
			Signer:             signer,
			DelegateEndpoint:   "http://example.com",
			ResourcePathPrefix: "/my/",
		}

		// signing
		responseWriter := httptest.NewRecorder()
		responseBody := handler.Sign("path", "get")
		// handler.Sign(responseWriter, &http.Request{URL: httputil.MustParse("/sign/my/path")})

		// responseBody := responseWriter.Body.String()

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
