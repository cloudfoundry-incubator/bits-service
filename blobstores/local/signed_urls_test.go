package local_test

import (
	"net/http"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo/pathsigner"
	"github.com/urfave/negroni"

	"net/http/httptest"

	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	. "github.com/petergtz/bitsgo/blobstores/local"
)

func TestLocalBlobstore(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "LocalBlobstore")
}

const (
	longerThanExpirationDuration = 2 * time.Hour
)

var _ = Describe("Signing URLs", func() {
	var (
		mockClock *clock.Mock
		signer    *pathsigner.PathSigner
		handler   *LocalResourceSigner
	)

	BeforeEach(func() {
		mockClock = clock.NewMock()
		signer = &pathsigner.PathSigner{"geheim", mockClock}
		handler = &LocalResourceSigner{
			Signer:             signer,
			DelegateEndpoint:   "http://example.com",
			ResourcePathPrefix: "/my/",
			Clock:              mockClock,
		}
	})

	It("signs and verifies URLs", func() {
		// signing
		responseBody := handler.Sign("path", "get")

		Expect(responseBody).To(ContainSubstring("http://example.com/my/path?md5="))
		Expect(responseBody).To(ContainSubstring("expires"))

		// verifying
		responseWriter := httptest.NewRecorder()
		delegateHandler := NewMockHandler()

		r := mux.NewRouter()
		r.Path("/my/path").Methods("GET").Handler(negroni.New(
			&SignatureVerificationMiddleware{signer},
			negroni.Wrap(delegateHandler),
		))
		r.ServeHTTP(responseWriter, httptest.NewRequest("GET", responseBody, nil))

		Expect(responseWriter.Code).To(Equal(http.StatusOK))
	})

	It("signs and verifies URLs", func() {
		// signing
		responseBody := handler.Sign("path", "get")

		Expect(responseBody).To(ContainSubstring("http://example.com/my/path?md5="))
		Expect(responseBody).To(ContainSubstring("expires"))

		mockClock.Add(longerThanExpirationDuration)

		// verifying
		responseWriter := httptest.NewRecorder()
		delegateHandler := NewMockHandler()

		r := mux.NewRouter()
		r.Path("/my/path").Methods("GET").Handler(negroni.New(
			&SignatureVerificationMiddleware{signer},
			negroni.Wrap(delegateHandler),
		))
		r.ServeHTTP(responseWriter, httptest.NewRequest("GET", responseBody, nil))

		Expect(responseWriter.Code).To(Equal(http.StatusForbidden))
	})

})
