package acceptance_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	bitsgo "github.com/cloudfoundry-incubator/bits-service"
	. "github.com/cloudfoundry-incubator/bits-service/acceptance_test"
	acceptance "github.com/cloudfoundry-incubator/bits-service/acceptance_test"
	"github.com/cloudfoundry-incubator/bits-service/httputil"
	. "github.com/cloudfoundry-incubator/bits-service/testutil"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
)

var client *http.Client = acceptance.CreateTLSClient("ca_cert")

func TestEndToEnd(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)

	CreateFakeEiriniFS()
	SetUpAndTearDownServer()
	ginkgo.RunSpecs(t, "EndToEnd HTTPS")
}

var _ = Describe("Accessing the bits-service", func() {
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
		It("returns http.StatusForbidden when accessing package through public host without signature", func() {
			Expect(client.Get("https://public.127.0.0.1.nip.io:4443/packages/notexistent")).
				To(WithTransform(GetStatusCode, Equal(http.StatusForbidden)))
		})

		Context("After retrieving a signed URL", func() {
			It("returns http.StatusOK when accessing package through public host with signature", func() {
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

			It("can POST a buildpack to a signed URL", func() {
				response, e := client.Do(
					newGetRequest("https://internal.127.0.0.1.nip.io:4443/sign/buildpacks?verb=post", "the-username", "the-password"))
				Expect(e).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				signedUrl, e := ioutil.ReadAll(response.Body)
				Expect(e).NotTo(HaveOccurred())

				r, e := httputil.NewPostRequest(string(signedUrl), map[string]map[string]io.Reader{
					"bits": map[string]io.Reader{"somefilename": CreateZip(map[string]string{"somefile": "lalala\n\n"})},
				})
				Expect(e).NotTo(HaveOccurred())
				response, e = client.Do(r)
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))

				r, e = httputil.NewPostRequest(string(signedUrl), map[string]map[string]io.Reader{
					"bits": map[string]io.Reader{"somefilename": CreateZip(map[string]string{"manifest.yml": "lalala\n\n"})},
				})
				Expect(e).NotTo(HaveOccurred())
				response, e = client.Do(r)
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))

				r, e = httputil.NewPostRequest(string(signedUrl), map[string]map[string]io.Reader{
					"bits": map[string]io.Reader{"somefilename": CreateZip(map[string]string{"manifest.yml": "stack: the-stack\n\n"})},
				})
				Expect(e).NotTo(HaveOccurred())
				response, e = client.Do(r)
				Expect(response.StatusCode).To(Equal(http.StatusCreated))

				responseBody, e := ioutil.ReadAll(response.Body)
				Expect(e).NotTo(HaveOccurred())

				var jsonBody bitsgo.ResponseBody
				e = json.Unmarshal(responseBody, &jsonBody)
				Expect(e).NotTo(HaveOccurred())

				Expect(jsonBody.Sha256).To(Equal("dc7e0bb2e12e1e566a00286ec2af5a82447f9522fa74a28a86d22a2ab09c9bb9"))

				response, e = client.Do(
					newGetRequest(fmt.Sprintf("https://internal.127.0.0.1.nip.io:4443/sign/buildpacks/%v/metadata", jsonBody.Guid), "the-username", "the-password"))
				Expect(e).ShouldNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				signedUrl, e = ioutil.ReadAll(response.Body)

				response, e = client.Get(string(signedUrl))
				Expect(e).NotTo(HaveOccurred())
				responseBody, e = ioutil.ReadAll(response.Body)
				Expect(e).NotTo(HaveOccurred())

				var bpMetdata bitsgo.BuildpackMetadata
				e = json.Unmarshal(responseBody, &bpMetdata)
				Expect(e).NotTo(HaveOccurred())
				Expect(bpMetdata.Key).To(Equal(jsonBody.Guid))
				Expect(bpMetdata.Sha1).To(Equal(jsonBody.Sha1))
				Expect(bpMetdata.Sha256).To(Equal(jsonBody.Sha256))
				Expect(bpMetdata.Stack).To(Equal("the-stack"))
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

	Describe("/droplets", func() {
		Describe("DELETE /droplets/:guid/:hash", func() {
			It("deletes OCI image artifacts too", func() {
				r, e := httputil.NewPutRequest("https://internal.127.0.0.1.nip.io:4443/droplets/the-droplet-guid/droplet-hash", map[string]map[string]io.Reader{
					"bits": map[string]io.Reader{"somefilename": CreateGZip(map[string]string{"somefile": "lalala\n\n"})},
				})
				r.SetBasicAuth("the-username", "the-password")
				response, e := client.Do(r)
				Expect(e).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated))

				response, e = client.Do(
					newGetRequest("https://internal.127.0.0.1.nip.io:4443/droplets/the-droplet-guid/droplet-hash", "the-username", "the-password"))
				Expect(e).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				responseBody, e := ioutil.ReadAll(response.Body)
				Expect(e).NotTo(HaveOccurred())
				ExpectTarGzipFileWithOneFileInIt(responseBody, "somefile", "lalala\n\n")

				response, e = client.Do(
					newGetRequest("https://registry.127.0.0.1.nip.io:4443/v2/cloudfoundry/the-droplet-guid/manifests/droplet-hash", "the-username", "the-password"))
				Expect(e).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				Expect(filesInDropletFolder()).To(HaveLen(5)) // droplet itself + OCI manifest index + OCI manifest + OCI config + OCI droplet layer blob

				response, e = client.Do(httputil.NewRequest("DELETE", "https://internal.127.0.0.1.nip.io:4443/droplets/the-droplet-guid/droplet-hash", nil).Build())
				Expect(e).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNoContent))

				response, e = client.Do(
					newGetRequest("https://registry.127.0.0.1.nip.io:4443/v2/cloudfoundry/the-droplet-guid/manifests/droplet-hash", "the-username", "the-password"))
				Expect(e).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))

				Expect(filesInDropletFolder()).To(BeEmpty())
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

func filesInDropletFolder() []string {
	var result []string
	filepath.Walk("/tmp/droplets", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			result = append(result, info.Name())
		}
		return nil
	})
	return result
}

func ExpectTarGzipFileWithOneFileInIt(body []byte, expectedFilename string, expectedFileContent string) {
	gzipReader, e := gzip.NewReader(bytes.NewReader(body))
	Expect(e).NotTo(HaveOccurred())
	defer gzipReader.Close()
	tarGzipReader := tar.NewReader(gzipReader)

	hdr, e := tarGzipReader.Next()
	Expect(e).NotTo(HaveOccurred())

	Expect(hdr.Name).To(Equal(expectedFilename))
	var fileContent bytes.Buffer
	_, e = io.Copy(&fileContent, tarGzipReader)
	Expect(e).NotTo(HaveOccurred())
	Expect(fileContent.String()).To(Equal(expectedFileContent))
}
