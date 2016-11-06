package main_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"mime/multipart"

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

	})

	AfterSuite(func() {
		if session != nil {
			session.Kill()
		}
		gexec.CleanupBuildArtifacts()
	})

	It("return 404 for a package that does not exist", func() {

		response, err := http.Get("http://internal.127.0.0.1.xip.io:8000/packages/notexistent")
		Ω(err).ShouldNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(404))

	})

	FIt("return 200 for a package that does exist", func() {
		bodyBuf := &bytes.Buffer{}
		multipartWriter := multipart.NewWriter(bodyBuf)
		formFileWriter, e := multipartWriter.CreateFormFile("package", "somefilename")
		Ω(e).ShouldNot(HaveOccurred())
		fmt.Fprint(formFileWriter, "My test string\n\n")
		multipartWriter.Close()

		request, e := http.NewRequest("PUT", "http://internal.127.0.0.1.xip.io:8000/packages/myguid", bodyBuf)
		Ω(e).ShouldNot(HaveOccurred())
		request.Header.Add("Content-Type", multipartWriter.FormDataContentType())

		response, e := new(http.Client).Do(request)
		Ω(e).ShouldNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(201))

		response, e = http.Get("http://internal.127.0.0.1.xip.io:8000/packages/myguid")
		Ω(e).ShouldNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(200))
	})

})
