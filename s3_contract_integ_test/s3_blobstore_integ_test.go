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
		config   S3BlobstoreConfig
		filepath string
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

		filepath = fmt.Sprintf("testfile-%v", time.Now())
	})

	Describe("S3LegacyBlobstore", func() {
		It("can put and get a resource there", func() {
			blobstore := NewS3LegacyBlobstore(config.Bucket, config.AccessKeyID, config.SecretAccessKey, config.Region)

			// Put:
			redirectLocation, e := blobstore.Put(filepath, strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())
			Expect(redirectLocation).To(BeEmpty())

			// Get:
			_, redirectLocation, e = blobstore.Get(filepath)
			Expect(e).NotTo(HaveOccurred())
			Expect(redirectLocation).NotTo(BeEmpty())

			// Follow redirect:
			response, e := http.Get(redirectLocation)
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(response.Body)).To(MatchRegexp("the file content"))

			// Delete:
			e = blobstore.Delete(filepath)
			Expect(e).NotTo(HaveOccurred())
		})
	})

	Describe("S3PureRedirectBlobstore", func() {
		It("can put and get a resource there", func() {
			blobstore := NewS3PureRedirectBlobstore(config.Bucket, config.AccessKeyID, config.SecretAccessKey, config.Region)

			// Put:
			redirectLocation, e := blobstore.Put(filepath, nil)
			Expect(e).NotTo(HaveOccurred())
			Expect(redirectLocation).NotTo(BeEmpty())

			// Follow redirect:
			request, e := http.NewRequest("PUT", redirectLocation, strings.NewReader("the file content"))
			Expect(e).NotTo(HaveOccurred())
			response, e := http.DefaultClient.Do(request)
			Expect(e).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			// Get:
			_, redirectLocation, e = blobstore.Get(filepath)
			Expect(e).NotTo(HaveOccurred())
			Expect(redirectLocation).NotTo(BeEmpty())

			// Follow redirect:
			response, e = http.Get(redirectLocation)
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(response.Body)).To(MatchRegexp("the file content"))

			// Delete:
			e = blobstore.Delete(filepath)
			Expect(e).NotTo(HaveOccurred())
		})
	})
})
