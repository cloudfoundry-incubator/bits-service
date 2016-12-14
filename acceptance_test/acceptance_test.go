package main_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"strings"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/petergtz/bitsgo/httputil"
)

func TestEndToEnd(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "EndToEnd")
}

var _ = Describe("Accessing the bits-service", func() {

	var session *gexec.Session

	BeforeSuite(func() {
		pathToWebserver, err := gexec.Build("github.com/petergtz/bitsgo")
		Ω(err).ShouldNot(HaveOccurred())

		session, err = gexec.Start(exec.Command(pathToWebserver, "--config", "config.yml"), GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())
		time.Sleep(200 * time.Millisecond)
		Expect(session.ExitCode()).To(Equal(-1), "Webserver error message: %s", string(session.Err.Contents()))
	})

	AfterSuite(func() {
		if session != nil {
			session.Kill()
		}
		gexec.CleanupBuildArtifacts()
	})

	Context("through private host", func() {
		It("return http.StatusNotFound for a package that does not exist", func() {
			Expect(http.Get("http://internal.127.0.0.1.xip.io:8000/packages/notexistent")).
				To(WithTransform(GetStatusCode, Equal(http.StatusNotFound)))
		})

		It("return http.StatusOK for a package that does exist", func() {
			request := NewPutRequest("http://internal.127.0.0.1.xip.io:8000/packages/myguid", map[string]map[string]io.Reader{
				"package": map[string]io.Reader{"somefilename": strings.NewReader("My test string")},
			})

			Expect(http.DefaultClient.Do(request)).To(WithTransform(GetStatusCode, Equal(201)))

			Expect(http.Get("http://internal.127.0.0.1.xip.io:8000/packages/myguid")).
				To(WithTransform(GetStatusCode, Equal(http.StatusOK)))
		})
	})

	Context("through public host", func() {
		It("returns http.StatusForbidden when accessing package through public host without md5", func() {
			Expect(http.Get("http://public.127.0.0.1.xip.io:8000/packages/notexistent")).
				To(WithTransform(GetStatusCode, Equal(http.StatusForbidden)))
		})

		Context("After retrieving a signed URL", func() {
			It("returns http.StatusOK when accessing package through public host with md5", func() {
				request := NewPutRequest("http://internal.127.0.0.1.xip.io:8000/packages/myguid", map[string]map[string]io.Reader{
					"package": map[string]io.Reader{"somefilename": strings.NewReader("lalala\n\n")},
				})
				Expect(http.DefaultClient.Do(request)).To(WithTransform(GetStatusCode, Equal(201)))

				response, e := http.Get("http://internal.127.0.0.1.xip.io:8000/sign/packages/myguid")
				Ω(e).ShouldNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				signedUrl, e := ioutil.ReadAll(response.Body)
				Ω(e).ShouldNot(HaveOccurred())

				response, e = http.Get(string(signedUrl))
				Ω(e).ShouldNot(HaveOccurred())
				Expect(ioutil.ReadAll(response.Body)).To(ContainSubstring("lalala"))
			})
		})
	})

})

func NewPutRequest(url string, formFiles map[string]map[string]io.Reader) *http.Request {
	if len(formFiles) > 1 {
		panic("More than one formFile is not supported yet")
	}
	bodyBuf := &bytes.Buffer{}
	request, e := http.NewRequest("PUT", url, bodyBuf)
	Ω(e).ShouldNot(HaveOccurred())
	header, e := httputil.AddFormFileTo(bodyBuf, formFiles)
	Ω(e).ShouldNot(HaveOccurred())
	httputil.AddHeaderTo(request, header)
	return request
}

func GetStatusCode(response *http.Response) int { return response.StatusCode }
