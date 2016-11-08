package main_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"time"

	"mime/multipart"

	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Signing URLs", func() {

	var session *gexec.Session

	BeforeSuite(func() {
		pathToWebserver, err := gexec.Build("github.com/petergtz/bitsgo")
		Ω(err).ShouldNot(HaveOccurred())

		session, err = gexec.Start(exec.Command(pathToWebserver), GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())
		time.Sleep(200 * time.Millisecond)
		Expect(session.ExitCode()).To(Equal(-1))
	})

	AfterSuite(func() {
		if session != nil {
			session.Kill()
		}
		gexec.CleanupBuildArtifacts()
	})

	It("return 404 for a package that does not exist", func() {
		Expect(http.Get("http://internal.127.0.0.1.xip.io:8000/packages/notexistent")).
			To(WithTransform(GetStatusCode, Equal(404)))
	})

	It("return 200 for a package that does exist", func() {
		request := NewPutRequest("http://internal.127.0.0.1.xip.io:8000/packages/myguid", map[string]map[string]io.Reader{
			"package": map[string]io.Reader{"somefilename": strings.NewReader("My test string")},
		})

		Expect(http.DefaultClient.Do(request)).To(WithTransform(GetStatusCode, Equal(201)))

		Expect(http.Get("http://internal.127.0.0.1.xip.io:8000/packages/myguid")).
			To(WithTransform(GetStatusCode, Equal(200)))
	})

	It("returns 403 when accessing package through public host without md5", func() {
		Expect(http.Get("http://public.127.0.0.1.xip.io:8000/packages/notexistent")).
			To(WithTransform(GetStatusCode, Equal(403)))
	})

	It("returns 200 when accessing package through public host with md5", func() {
		request := NewPutRequest("http://internal.127.0.0.1.xip.io:8000/packages/myguid", map[string]map[string]io.Reader{
			"package": map[string]io.Reader{"somefilename": strings.NewReader("lalala\n\n")},
		})
		Expect(http.DefaultClient.Do(request)).To(WithTransform(GetStatusCode, Equal(201)))

		response, e := http.Get("http://internal.127.0.0.1.xip.io:8000/sign/packages/myguid")
		Ω(e).ShouldNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(200))

		signedUrl, e := ioutil.ReadAll(response.Body)
		Ω(e).ShouldNot(HaveOccurred())

		response, e = http.Get(string(signedUrl))
		Ω(e).ShouldNot(HaveOccurred())
		Expect(ioutil.ReadAll(response.Body)).To(ContainSubstring("lalala"))
	})

})

func NewPutRequest(url string, formFiles map[string]map[string]io.Reader) *http.Request {
	if len(formFiles) > 1 {
		panic("More than one formFile is not supported yet")
	}
	bodyBuf := &bytes.Buffer{}
	request, e := http.NewRequest("PUT", url, bodyBuf)
	Ω(e).ShouldNot(HaveOccurred())
	for name, fileAndReader := range formFiles {
		multipartWriter := multipart.NewWriter(bodyBuf)
		for file, reader := range fileAndReader {
			formFileWriter, e := multipartWriter.CreateFormFile(name, file)
			Ω(e).ShouldNot(HaveOccurred())
			io.Copy(formFileWriter, reader)
			multipartWriter.Close()
			request.Header.Add("Content-Type", multipartWriter.FormDataContentType())
		}
	}
	return request
}

func GetStatusCode(response *http.Response) int { return response.StatusCode }
