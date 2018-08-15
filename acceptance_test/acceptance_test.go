package main_test

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/cloudfoundry-incubator/bits-service/httputil"
	. "github.com/cloudfoundry-incubator/bits-service/testutil"
)

func TestEndToEnd(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "EndToEnd")
}

var _ = Describe("Accessing the bits-service", func() {

	var (
		session *gexec.Session
		client  *http.Client
	)

	BeforeSuite(func() {
		pathToWebserver, err := gexec.Build("github.com/cloudfoundry-incubator/bits-service/cmd/bitsgo")
		Ω(err).ShouldNot(HaveOccurred())

		os.Setenv("BITS_LISTEN_ADDR", "127.0.0.1")
		session, err = gexec.Start(exec.Command(pathToWebserver, "--config", "config.yml"), GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())
		time.Sleep(200 * time.Millisecond)
		Expect(session.ExitCode()).To(Equal(-1), "Webserver error message: %s", string(session.Err.Contents()))

		caCert, err := ioutil.ReadFile("ca_cert")
		Ω(err).ShouldNot(HaveOccurred())
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
			RootCAs: caCertPool,
		}}}
	})

	AfterSuite(func() {
		if session != nil {
			session.Kill()
		}
		gexec.CleanupBuildArtifacts()
	})

	Context("through private host", func() {
		It("return http.StatusNotFound for a package that does not exist", func() {
			Expect(client.Get("https://internal.127.0.0.1.nip.io:4443/packages/notexistent")).
				To(WithTransform(GetStatusCode, Equal(http.StatusNotFound)))
		})

		It("return http.StatusOK for a package that does exist", func() {
			request, e := httputil.NewPutRequest("https://internal.127.0.0.1.nip.io:4443/packages/myguid", map[string]map[string]io.Reader{
				"package": map[string]io.Reader{"somefilename": CreateZip(map[string]string{"somefile": "lalala\n\n"})},
			})
			Expect(e).NotTo(HaveOccurred())

			Expect(client.Do(request)).To(WithTransform(GetStatusCode, Equal(201)))

			Expect(client.Get("https://internal.127.0.0.1.nip.io:4443/packages/myguid")).
				To(WithTransform(GetStatusCode, Equal(http.StatusOK)))
		})
	})

	Context("through public host", func() {
		It("returns http.StatusForbidden when accessing package through public host without md5", func() {
			Expect(client.Get("https://public.127.0.0.1.nip.io:4443/packages/notexistent")).
				To(WithTransform(GetStatusCode, Equal(http.StatusForbidden)))
		})

		Context("After retrieving a signed URL", func() {
			It("returns http.StatusOK when accessing package through public host with md5", func() {
				request, e := httputil.NewPutRequest("https://internal.127.0.0.1.nip.io:4443/packages/myguid", map[string]map[string]io.Reader{
					"package": map[string]io.Reader{"somefilename": CreateZip(map[string]string{"somefile": "lalala\n\n"})},
				})
				Expect(e).NotTo(HaveOccurred())

				Expect(client.Do(request)).To(WithTransform(GetStatusCode, Equal(201)))

				response, e := client.Do(
					newGetRequest("https://internal.127.0.0.1.nip.io:4443/sign/packages/myguid", "the-username", "the-password"))
				Ω(e).ShouldNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				signedUrl, e := ioutil.ReadAll(response.Body)
				Ω(e).ShouldNot(HaveOccurred())
				response, e = client.Get(string(signedUrl))
				Ω(e).ShouldNot(HaveOccurred())

				responseBody, e := ioutil.ReadAll(response.Body)
				Expect(e).NotTo(HaveOccurred())
				zipReader, e := zip.NewReader(bytes.NewReader(responseBody), int64(len(responseBody)))
				Expect(e).NotTo(HaveOccurred())

				Expect(zipReader.File).To(HaveLen(1))
				VerifyZipFileEntry(zipReader, "somefile", "lalala\n\n")
			})
		})
	})

	Describe("/packages", func() {
		Describe("PUT", func() {
			Context("async=true", func() {
				It("returns StatusAccepted", func() {
					response, e := client.Do(
						newGetRequest("https://internal.127.0.0.1.nip.io:4443/sign/packages/myguid?verb=put", "the-username", "the-password"))
					Expect(e).NotTo(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusOK))

					signedUrl, e := ioutil.ReadAll(response.Body)
					Expect(e).NotTo(HaveOccurred())

					r, e := httputil.NewPutRequest(string(signedUrl)+"&async=true", map[string]map[string]io.Reader{
						"package": map[string]io.Reader{"somefilename": CreateZip(map[string]string{"somefile": "lalala\n\n"})},
					})
					Expect(e).NotTo(HaveOccurred())
					response, e = client.Do(r)

					Expect(e).NotTo(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusAccepted))
				})
			})
		})
	})
})

func newGetRequest(url string, username string, password string) *http.Request {
	request, e := http.NewRequest("GET", url, nil)
	Expect(e).NotTo(HaveOccurred())
	request.SetBasicAuth(username, password)
	return request
}

func GetStatusCode(response *http.Response) int { return response.StatusCode }
