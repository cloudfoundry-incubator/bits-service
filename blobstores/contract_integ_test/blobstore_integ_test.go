package main_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/blobstores/alibaba"
	"github.com/petergtz/bitsgo/blobstores/azure"
	"github.com/petergtz/bitsgo/blobstores/gcp"
	"github.com/petergtz/bitsgo/blobstores/openstack"
	"github.com/petergtz/bitsgo/blobstores/s3"
	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/httputil"
)

var _ = Describe("Non-local blobstores", func() {

	var (
		filepath     string
		srcFilepath  string
		destFilepath string
		blobstore    blobstore
	)

	itCanPutAndGetAResourceThere := func() {

		It("can put and get a resource there", func() {
			redirectLocation, e := blobstore.HeadOrRedirectAsGet(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(http.Get(redirectLocation)).To(HaveStatusCode(http.StatusNotFound))

			body, e := blobstore.Get(filepath)
			Expect(e).To(BeAssignableToTypeOf(&bitsgo.NotFoundError{}))
			Expect(body).To(BeNil())

			body, redirectLocation, e = blobstore.GetOrRedirect(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(body).To(BeNil())
			Expect(http.Get(redirectLocation)).To(HaveStatusCode(http.StatusNotFound))

			e = blobstore.Put(filepath, strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())

			redirectLocation, e = blobstore.HeadOrRedirectAsGet(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(http.Get(redirectLocation)).To(HaveStatusCode(http.StatusOK))

			body, e = blobstore.Get(filepath)
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(body)).To(ContainSubstring("the file content"))

			body, redirectLocation, e = blobstore.GetOrRedirect(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(body).To(BeNil())
			Expect(http.Get(redirectLocation)).To(HaveBodyWithSubstring("the file content"))

			e = blobstore.Delete(filepath)
			Expect(e).NotTo(HaveOccurred())

			redirectLocation, e = blobstore.HeadOrRedirectAsGet(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(http.Get(redirectLocation)).To(HaveStatusCode(http.StatusNotFound))

			body, e = blobstore.Get(filepath)
			Expect(e).To(BeAssignableToTypeOf(&bitsgo.NotFoundError{}))
			Expect(body).To(BeNil())

			body, redirectLocation, e = blobstore.GetOrRedirect(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(body).To(BeNil())
			Expect(http.Get(redirectLocation)).To(HaveStatusCode(http.StatusNotFound))
		})

		Describe("DeleteDir", func() {
			BeforeEach(func() {
				e := blobstore.Put("one", strings.NewReader("the file content"))
				Expect(e).NotTo(HaveOccurred())

				e = blobstore.Put("two", strings.NewReader("the file content"))
				Expect(e).NotTo(HaveOccurred())

				Expect(blobstore.Exists("one")).To(BeTrue())
				Expect(blobstore.Exists("two")).To(BeTrue())
			})

			AfterEach(func() {
				blobstore.Delete("one")
				blobstore.Delete("two")
				Expect(blobstore.Exists("one")).To(BeFalse())
				Expect(blobstore.Exists("two")).To(BeFalse())
			})

			It("Can delete a prefix", func() {
				e := blobstore.DeleteDir("")
				Expect(e).NotTo(HaveOccurred())

				Expect(blobstore.Exists("one")).To(BeFalse())
				Expect(blobstore.Exists("two")).To(BeFalse())
			})
		})

		Context("Can copy", func() {

			BeforeEach(func() {
				srcFilepath = fmt.Sprintf("src-testfile")
				destFilepath = fmt.Sprintf("dest-testfile")
				body, e := blobstore.Get(srcFilepath)
				Expect(e).To(BeAssignableToTypeOf(&bitsgo.NotFoundError{}))
				Expect(body).To(BeNil())
				e = blobstore.Put(srcFilepath, strings.NewReader("the file content"))
				Expect(e).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				//teardown
				e := blobstore.Delete(srcFilepath)
				Expect(e).NotTo(HaveOccurred())
				e = blobstore.Delete(destFilepath)
				Expect(e).NotTo(HaveOccurred())
			})
			It("a resource from src to dest", func() {
				e := blobstore.Copy(srcFilepath, destFilepath)
				Expect(e).NotTo(HaveOccurred())

				body, e := blobstore.Get(destFilepath)
				Expect(e).NotTo(HaveOccurred())
				Expect(body).NotTo(BeNil())

			})
		})

		It("Can delete a prefix like in a file tree", func() {
			Expect(blobstore.Exists("dir/one")).To(BeFalse())
			Expect(blobstore.Exists("dir/two")).To(BeFalse())

			e := blobstore.Put("dir/one", strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())
			e = blobstore.Put("dir/two", strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())

			Expect(blobstore.Exists("dir/one")).To(BeTrue())
			Expect(blobstore.Exists("dir/two")).To(BeTrue())

			e = blobstore.DeleteDir("dir")
			Expect(e).NotTo(HaveOccurred())

			Expect(blobstore.Exists("dir/one")).To(BeFalse())
			Expect(blobstore.Exists("dir/two")).To(BeFalse())
		})

		It("can get a signed PUT URL and upload something to it", func() {
			signedUrl := blobstore.Sign(filepath, "put", time.Now().Add(1*time.Hour))

			r := httputil.NewRequest("PUT", signedUrl, strings.NewReader("the file content"))

			// The following line is a hack to make Azure work.
			// (See:
			// https://stackoverflow.com/questions/37824136/put-on-sas-blob-url-without-specifying-x-ms-blob-type-header
			// https://stackoverflow.com/questions/16160045/azure-rest-webclient-put-blob
			// https://stackoverflow.com/questions/12711150/unable-to-upload-file-image-n-vdo-to-blob-storage-getting-error-mandatory-he)

			// Not a huge problem, since we decided that all uploads must go through the bits-service anyway. But still annoying.
			r.WithHeader("x-ms-blob-type", "BlockBlob")

			response, e := http.DefaultClient.Do(r.Build())
			Expect(e).NotTo(HaveOccurred())

			Expect(response.StatusCode).To(Or(Equal(http.StatusOK), Equal(http.StatusCreated)))
		})
	}

	ItDoesNotReturnNotFoundError := func() {
		It("does not throw a NotFoundError", func() {
			_, e := blobstore.Get("irrelevant-path")
			Expect(e).NotTo(BeAssignableToTypeOf(&bitsgo.NotFoundError{}))
		})
	}

	var configFileContent []byte

	BeforeEach(func() {
		filename := os.Getenv("CONFIG")
		if filename == "" {
			fmt.Println("No $CONFIG set. Defaulting to integration_test_config.yml")
			filename = "integration_test_config.yml"
		}
		file, e := os.Open(filename)
		Expect(e).NotTo(HaveOccurred())
		defer file.Close()
		configFileContent, e = ioutil.ReadAll(file)
		Expect(e).NotTo(HaveOccurred())

		filepath = fmt.Sprintf("testfile-%v", time.Now())
	})

	Context("S3", func() {
		var s3Config config.S3BlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &s3Config)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = s3.NewBlobstore(s3Config) })

		itCanPutAndGetAResourceThere()

		Context("With non-existing bucket", func() {
			BeforeEach(func() { s3Config.Bucket += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})

	})

	Context("GCP", func() {
		var gcpConfig config.GCPBlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &gcpConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = gcp.NewBlobstore(gcpConfig) })

		itCanPutAndGetAResourceThere()

		Context("With non-existing bucket", func() {
			BeforeEach(func() { gcpConfig.Bucket += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})

	})

	Context("azure", func() {
		var azureConfig config.AzureBlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &azureConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = azure.NewBlobstore(azureConfig) })

		itCanPutAndGetAResourceThere()

		Context("With non-existing bucket", func() {
			BeforeEach(func() { azureConfig.ContainerName += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})

	})

	Context("openstack", func() {
		var openstackConfig config.OpenstackBlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &openstackConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = openstack.NewBlobstore(openstackConfig) })

		itCanPutAndGetAResourceThere()

		Context("With non-existing bucket", func() {
			BeforeEach(func() { openstackConfig.ContainerName += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})

	})
	FContext("alibaba", func() {
		var alibabaConfig config.AlibabaBlobstoreConfig
		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &alibabaConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = alibaba.NewBlobstore(alibabaConfig) })

		itCanPutAndGetAResourceThere()

		Context("With non-existing bucket", func() {
			BeforeEach(func() { alibabaConfig.BucketName += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})

	})
})

func HaveBodyWithSubstring(substring string) types.GomegaMatcher {
	return WithTransform(func(response *http.Response) string {
		actualBytes, e := ioutil.ReadAll(response.Body)
		if e != nil {
			panic(e)
		}
		response.Body.Close()
		return string(actualBytes)
	}, Equal(substring))
}

func HaveStatusCode(statusCode int) types.GomegaMatcher {
	return WithTransform(func(response *http.Response) int {
		return response.StatusCode
	}, Equal(statusCode))
}
