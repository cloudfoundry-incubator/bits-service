package main_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	yaml "gopkg.in/yaml.v2"

	"crypto/md5"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/blobstores/azure"
	"github.com/petergtz/bitsgo/blobstores/gcp"
	"github.com/petergtz/bitsgo/blobstores/s3"
	"github.com/petergtz/bitsgo/config"
)

func TestGCPBlobstore(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "GCP Blobstore Contract Integration")
}

type blobstore interface {
	// Can't do the following until it is added in Go: (See also https://github.com/golang/go/issues/6977)
	// routes.Blobstore
	// routes.NoRedirectBlobstore

	// Instead doing:
	bitsgo.Blobstore
	Get(path string) (body io.ReadCloser, err error)
}

var _ = Describe("Non-local blobstores", func() {
	var (
		filepath  string
		blobstore blobstore
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

		XIt("can upload a 500MB file", func() {
			filename, expectedMd5sum := createTempFileWithRandomContent(500 << 20)
			defer os.Remove(filename)

			file, e := os.Open(filename)
			Expect(e).NotTo(HaveOccurred())
			defer file.Close()

			e = blobstore.Put(filepath, file)
			Expect(e).NotTo(HaveOccurred())
			defer blobstore.Delete(filepath)

			reader, e := blobstore.Get(filepath)
			Expect(e).NotTo(HaveOccurred())
			defer reader.Close()

			Expect(md5sum(reader)).To(Equal(expectedMd5sum))
		})

		It("Can delete a prefix", func() {
			Expect(blobstore.Exists("one")).To(BeFalse())
			Expect(blobstore.Exists("two")).To(BeFalse())

			e := blobstore.Put("one", strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())

			e = blobstore.Put("two", strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())

			Expect(blobstore.Exists("one")).To(BeTrue())
			Expect(blobstore.Exists("two")).To(BeTrue())

			e = blobstore.DeleteDir("")
			Expect(e).NotTo(HaveOccurred())

			Expect(blobstore.Exists("one")).To(BeFalse())
			Expect(blobstore.Exists("two")).To(BeFalse())
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

		Context("With existing bucket", func() {
			itCanPutAndGetAResourceThere()
		})

		Context("With non-existing bucket", func() {
			BeforeEach(func() { s3Config.Bucket += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})

	})

	Context("GCP", func() {
		var gcpConfig config.GCPBlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &gcpConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = gcp.NewBlobstore(gcpConfig) })

		Context("With existing bucket", func() {
			itCanPutAndGetAResourceThere()
		})

		Context("With non-existing bucket", func() {
			BeforeEach(func() { gcpConfig.Bucket += "non-existing" })

			ItDoesNotReturnNotFoundError()
		})

	})

	Context("azure", func() {
		var azureConfig config.AzureBlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &azureConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = azure.NewBlobstore(azureConfig) })

		Context("With existing bucket", func() {
			itCanPutAndGetAResourceThere()
		})

		Context("With non-existing bucket", func() {
			BeforeEach(func() { azureConfig.ContainerName += "non-existing" })

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

func createTempFileWithRandomContent(size int64) (filename string, md5sum []byte) {
	file, e := ioutil.TempFile("", "")
	Expect(e).NotTo(HaveOccurred())
	defer file.Close()

	md5Expected := md5.New()
	_, e = io.CopyN(io.MultiWriter(file, md5Expected), rand.New(rand.NewSource(time.Now().Unix())), size)
	Expect(e).NotTo(HaveOccurred())

	return file.Name(), md5Expected.Sum(nil)
}

func md5sum(reader io.Reader) []byte {
	md5Actual := md5.New()
	_, e := io.Copy(md5Actual, reader)
	Expect(e).NotTo(HaveOccurred())
	return md5Actual.Sum((nil))
}
