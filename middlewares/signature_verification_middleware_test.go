package middlewares_test

import (
	"net/http"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cloudfoundry-incubator/bits-service/pathsigner"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"

	"net/http/httptest"

	"time"

	. "github.com/cloudfoundry-incubator/bits-service/blobstores/local"
	. "github.com/cloudfoundry-incubator/bits-service/middlewares"
	"github.com/onsi/ginkgo"

	"github.com/onsi/gomega"
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
		mockClock           *clock.Mock
		pathSignerValidator *pathsigner.PathSignerValidator
		handler             *LocalResourceSigner
	)

	BeforeEach(func() {
		mockClock = clock.NewMock()
		pathSignerValidator = pathsigner.Validate(&pathsigner.PathSignerValidator{"geheim", mockClock, nil, ""})
		handler = &LocalResourceSigner{
			Signer:             pathSignerValidator,
			DelegateEndpoint:   "http://example.com",
			ResourcePathPrefix: "/my/",
		}
	})

	It("signs and verifies URLs", func() {
		// signing
		responseBody := handler.Sign("path", "get", mockClock.Now().Add(1*time.Hour))

		Expect(responseBody).To(ContainSubstring("http://example.com/my/path?signature="))
		Expect(responseBody).To(ContainSubstring("expires"))

		// verifying
		responseWriter := httptest.NewRecorder()
		delegateHandler := NewMockHandler()

		r := mux.NewRouter()
		r.Path("/my/path").Methods("GET").Handler(negroni.New(
			&SignatureVerificationMiddleware{pathSignerValidator},
			negroni.Wrap(delegateHandler),
		))
		r.ServeHTTP(responseWriter, httptest.NewRequest("GET", responseBody, nil))

		Expect(responseWriter.Code).To(Equal(http.StatusOK))
	})

	It("signs and returns an error when URL has expired", func() {
		// signing
		responseBody := handler.Sign("path", "get", mockClock.Now().Add(1*time.Hour))

		Expect(responseBody).To(ContainSubstring("http://example.com/my/path?signature="))
		Expect(responseBody).To(ContainSubstring("expires"))

		mockClock.Add(longerThanExpirationDuration)

		// verifying
		responseWriter := httptest.NewRecorder()
		delegateHandler := NewMockHandler()

		r := mux.NewRouter()
		r.Path("/my/path").Methods("GET").Handler(negroni.New(
			&SignatureVerificationMiddleware{pathSignerValidator},
			negroni.Wrap(delegateHandler),
		))
		r.ServeHTTP(responseWriter, httptest.NewRequest("GET", responseBody, nil))

		Expect(responseWriter.Code).To(Equal(http.StatusForbidden))
	})

})
