package acceptance_test

import (
	"net/http"
	"testing"

	. "github.com/cloudfoundry-incubator/bits-service/acceptance_test"
	acceptance "github.com/cloudfoundry-incubator/bits-service/acceptance_test"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
)

var client = &http.Client{}

func TestEndToEnd(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	CreateFakeEiriniFS()

	acceptance.SetUpAndTearDownServer()
	ginkgo.RunSpecs(t, "EndToEnd Identical registry and public hostname")
}

var _ = Describe("Accessing the bits-service", func() {
	Context("when public and registry endpoint use the same hostname", func() {
		Context("accessing non-exiting package through public host", func() {
			It("gets a status forbidden from the signature verification middleware", func() {
				Expect(client.Get("http://public-and-registry.127.0.0.1.nip.io:8888/packages/notexistent")).
					To(WithTransform(GetStatusCode, Equal(http.StatusForbidden)))
			})
		})

		Context("accessing OCI /v2 endpoint through registry host", func() {
			It("gets an HTTP Status OK", func() {
				req, err := http.NewRequest("GET", "http://public-and-registry.127.0.0.1.nip.io:8888/v2/", nil)
				Expect(err).ToNot(HaveOccurred())

				req.SetBasicAuth("the-username", "the-password")
				Expect(client.Do(req)).To(WithTransform(GetStatusCode, Equal(http.StatusOK)))
			})
		})
	})
})
