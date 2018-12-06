package acceptance_test

import (
	"io"
	"net/http"
	"testing"

	acceptance "github.com/cloudfoundry-incubator/bits-service/acceptance_test"
	"github.com/cloudfoundry-incubator/bits-service/httputil"
	. "github.com/cloudfoundry-incubator/bits-service/testutil"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	session *gexec.Session
	client  *http.Client
)

func TestEndToEnd(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)

	BeforeSuite(func() {
		session = acceptance.StartServer("config.yml")
		client = &http.Client{}
	})

	AfterSuite(func() {
		if session != nil {
			session.Kill()
		}
		gexec.CleanupBuildArtifacts()
	})

	ginkgo.RunSpecs(t, "EndToEnd HTTP")
}

var _ = Describe("Accessing the bits-service through HTTP", func() {
	Context("through private host", func() {
		Context("HTTP", func() {

			It("return http.StatusOK for a package that does exist", func() {
				request, e := httputil.NewPutRequest("http://internal.127.0.0.1.nip.io:8888/packages/myguid", map[string]map[string]io.Reader{
					"package": map[string]io.Reader{"somefilename": CreateZip(map[string]string{"somefile": "lalala\n\n"})},
				})
				Expect(e).NotTo(HaveOccurred())

				Expect(client.Do(request)).To(WithTransform(GetStatusCode, Equal(201)))

				Expect(client.Get("http://internal.127.0.0.1.nip.io:8888/packages/myguid")).
					To(WithTransform(GetStatusCode, Equal(http.StatusOK)))
			})
		})

		Context("HTTPS", func() {
			It("return http.StatusOK for a package that does exist", func() {
				client = acceptance.CreateTLSClient("../ca_cert")

				request, e := httputil.NewPutRequest("https://internal.127.0.0.1.nip.io:4444/packages/myguid", map[string]map[string]io.Reader{
					"package": map[string]io.Reader{"somefilename": CreateZip(map[string]string{"somefile": "lalala\n\n"})},
				})
				Expect(e).NotTo(HaveOccurred())

				Expect(client.Do(request)).To(WithTransform(GetStatusCode, Equal(201)))

				Expect(client.Get("https://internal.127.0.0.1.nip.io:4444/packages/myguid")).
					To(WithTransform(GetStatusCode, Equal(http.StatusOK)))
			})
		})

	})
})

func GetStatusCode(response *http.Response) int { return response.StatusCode }
