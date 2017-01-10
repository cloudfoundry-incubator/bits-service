package main_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	yaml "gopkg.in/yaml.v2"

	"os"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/petergtz/bitsgo/routes"
	. "github.com/petergtz/bitsgo/s3_blobstore"
)

type S3BlobstoreConfig struct {
	Bucket          string
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	Region          string
}

func TestS3Blobstore(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "S3 Blobstore Contract Integration")
}

var _ = Describe("S3 Blobstores", func() {
	var (
		config    S3BlobstoreConfig
		filepath  string
		blobstore routes.Blobstore
	)

	BeforeEach(func() {
		filename := os.Getenv("CONFIG")
		if filename == "" {
			fmt.Println("No $CONFIG set. Defaulting to integration_test_config.yml")
			filename = "integration_test_config.yml"
		}
		file, e := os.Open(filename)
		Expect(e).NotTo(HaveOccurred())
		defer file.Close()
		content, e := ioutil.ReadAll(file)
		Expect(e).NotTo(HaveOccurred())
		e = yaml.Unmarshal(content, &config)
		Expect(e).NotTo(HaveOccurred())
		Expect(config.Bucket).NotTo(BeEmpty())
		Expect(config.AccessKeyID).NotTo(BeEmpty())
		Expect(config.SecretAccessKey).NotTo(BeEmpty())
		Expect(config.Region).NotTo(BeEmpty())

		filepath = fmt.Sprintf("testfile-%v", time.Now())
	})

	Describe("S3NoRedirectBlobStore", func() {
		It("can put and get a resource there", func() {
			blobstore = NewS3NoRedirectBlobStore(config.Bucket, config.AccessKeyID, config.SecretAccessKey, config.Region)

			redirectLocation, e := blobstore.Head(filepath)
			Expect(e).To(BeAssignableToTypeOf(&routes.NotFoundError{}))
			Expect(redirectLocation).To(BeEmpty())

			body, redirectLocation, e := blobstore.Get(filepath)
			Expect(e).To(BeAssignableToTypeOf(&routes.NotFoundError{}))
			Expect(redirectLocation).To(BeEmpty())
			Expect(body).To(BeNil())

			redirectLocation, e = blobstore.Put(filepath, strings.NewReader("the file content"))
			Expect(redirectLocation, e).To(BeEmpty())

			redirectLocation, e = blobstore.Head(filepath)
			Expect(redirectLocation, e).To(BeEmpty())

			body, redirectLocation, e = blobstore.Get(filepath)
			Expect(redirectLocation, e).To(BeEmpty())
			Expect(ioutil.ReadAll(body)).To(ContainSubstring("the file content"))

			e = blobstore.Delete(filepath)
			Expect(e).NotTo(HaveOccurred())

			redirectLocation, e = blobstore.Head(filepath)
			Expect(e).To(BeAssignableToTypeOf(&routes.NotFoundError{}))
			Expect(redirectLocation).To(BeEmpty())

			body, redirectLocation, e = blobstore.Get(filepath)
			Expect(e).To(BeAssignableToTypeOf(&routes.NotFoundError{}))
			Expect(redirectLocation).To(BeEmpty())
			Expect(body).To(BeNil())
		})

	})

	Describe("S3PureRedirectBlobstore", func() {
		It("can put and get a resource there", func() {
			blobstore := NewS3PureRedirectBlobstore(config.Bucket, config.AccessKeyID, config.SecretAccessKey, config.Region)

			redirectLocation, e := blobstore.Head(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(http.Head(redirectLocation)).To(HaveStatusCode(http.StatusNotFound))

			body, redirectLocation, e := blobstore.Get(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(body).To(BeNil())
			Expect(http.Get(redirectLocation)).To(HaveStatusCode(http.StatusNotFound))

			redirectLocation, e = blobstore.Put(filepath, nil)
			Expect(redirectLocation, e).NotTo(BeEmpty())

			request, e := http.NewRequest("PUT", redirectLocation, strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())
			Expect(http.DefaultClient.Do(request)).To(HaveStatusCode(http.StatusOK))

			redirectLocation, e = blobstore.Head(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(http.Head(redirectLocation)).To(HaveStatusCode(http.StatusOK))

			body, redirectLocation, e = blobstore.Get(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(body).To(BeNil())
			Expect(http.Get(redirectLocation)).To(HaveBodyWithSubstring("the file content"))

			e = blobstore.Delete(filepath)
			Expect(e).NotTo(HaveOccurred())

			redirectLocation, e = blobstore.Head(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(http.Head(redirectLocation)).To(HaveStatusCode(http.StatusNotFound))

			body, redirectLocation, e = blobstore.Get(filepath)
			Expect(redirectLocation, e).NotTo(BeEmpty())
			Expect(body).To(BeNil())
			Expect(http.Get(redirectLocation)).To(HaveStatusCode(http.StatusNotFound))
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
