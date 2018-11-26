package config_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	. "github.com/cloudfoundry-incubator/bits-service/config"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Config")
}

const dummyBlobstoreConfigs = `
packages:
  blobstore_type: local
  local_config:
    path_prefix: dummy
droplets:
  blobstore_type: google
  gcp_config:
    bucket: dummy
buildpacks:
  blobstore_type: aws
  s3_config:
    bucket: dummy
app_stash:
  blobstore_type: webdav
  webdav_config:
    directory_key: dummy
`

var _ = Describe("config", func() {

	var configFile *os.File

	BeforeEach(func() {
		var e error
		configFile, e = ioutil.TempFile("", "bitsgo_config.yml")
		Expect(e).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if configFile != nil {
			configFile.Close()
			os.Remove(configFile.Name())
		}
	})

	It("can be loaded from yml file", func() {
		fmt.Fprintf(configFile, "%s", `
privatebuildpacks:
droplets:
packages:
app_stash:
logging:
  file: /tmp/bits-service.log
  syslog: vcap.bits-service
  level: debug
public_endpoint: https://public.127.0.0.1.nip.io
private_endpoint: https://internal.127.0.0.1.nip.io
secret: geheim
port: 8000
key_file: /some/path
cert_file: /some/path
`+
			dummyBlobstoreConfigs)
		config, e := LoadConfig(configFile.Name())

		Expect(e).NotTo(HaveOccurred())
		Expect(config.Secret).To(Equal("geheim"))
	})

	It("returns an error when file does not exist", func() {
		_, e := LoadConfig("non-existing.yml")

		Expect(e).To(HaveOccurred())
		Expect(e.Error()).To(ContainSubstring("error opening config"))
	})

	It("returns an error when file cannot be parsed", func() {
		fmt.Fprintf(configFile, "%s", `port: invalid_type`)

		_, e := LoadConfig(configFile.Name())

		Expect(e).To(HaveOccurred())
		Expect(e.Error()).To(ContainSubstring("error parsing config"))
	})

	It("returns an error when config values are invalid", func() {
		fmt.Fprintf(configFile, "")

		_, e := LoadConfig(configFile.Name())

		Expect(e).To(HaveOccurred())
		Expect(e.Error()).To(SatisfyAll(
			ContainSubstring("error in config"),
			ContainSubstring("port must be an integer > 0"),
			ContainSubstring("public_endpoint must not be empty"),
			ContainSubstring("private_endpoint must not be empty"),
		))
	})

	It("correctly converts max_body_size", func() {
		Expect((&BlobstoreConfig{MaxBodySize: `13M`}).MaxBodySizeBytes()).To(Equal(uint64(13631488)))
	})

	It("returns an error when max_body_size is invalid", func() {
		fmt.Fprintf(configFile, "%s", `max_body_size: 13 mb`)

		_, e := LoadConfig(configFile.Name())

		Expect(e).To(HaveOccurred())
		Expect(e.Error()).To(
			ContainSubstring("max_body_size is invalid"),
		)
	})

	It("returns 0 when max_body_size is not defined", func() {
		Expect((&BlobstoreConfig{}).MaxBodySizeBytes()).To(Equal(uint64(0)))
	})

	It("uses global value, when blobstore specific value is not set", func() {
		Expect((&BlobstoreConfig{GlobalMaxBodySize: `13MB`}).MaxBodySizeBytes()).To(Equal(uint64(13631488)))
	})

	It("correctly inherits global max_body_size when not configured in blobstore specifically", func() {
		fmt.Fprintf(configFile, "%s", `
privatebuildpacks:
droplets:
packages:
  max_body_size: 20MB
app_stash:
logging:
  file: /tmp/bits-service.log
  syslog: vcap.bits-service
  level: debug
public_endpoint: https://public.127.0.0.1.nip.io
private_endpoint: https://internal.127.0.0.1.nip.io
secret: geheim
port: 8000
max_body_size: 13M
key_file: /some/path
cert_file: /some/path
`+
			dummyBlobstoreConfigs)
		config, e := LoadConfig(configFile.Name())

		Expect(e).NotTo(HaveOccurred())
		Expect(config.Droplets.MaxBodySizeBytes()).To(Equal(uint64(13631488)))
		Expect(config.Buildpacks.MaxBodySizeBytes()).To(Equal(uint64(13631488)))
		Expect(config.AppStash.MaxBodySizeBytes()).To(Equal(uint64(13631488)))
		Expect(config.Packages.MaxBodySizeBytes()).To(Equal(uint64(20971520)))
	})

	Context("can read limits for resources match ", func() {
		It("value: MinimumSize", func() {
			fmt.Fprintf(configFile, "%s", `
public_endpoint: https://public.127.0.0.1.nip.io
private_endpoint: https://internal.127.0.0.1.nip.io
port: 8000
secret: geheim
key_file: /some/path
cert_file: /some/path
app_stash_config:
  minimum_size: 64K
  maximum_size: 13M
`+
				dummyBlobstoreConfigs)
			config, e := LoadConfig(configFile.Name())
			Expect(e).NotTo(HaveOccurred())
			Expect(config.AppStashConfig.MinimumSizeBytes()).To(Equal(uint64(65536)))
			Expect(config.AppStashConfig.MaximumSizeBytes()).To(Equal(uint64(13631488)))
		})

		Context("maximum_size is smaller than minimum_size", func() {

			It("returns an error", func() {
				fmt.Fprintf(configFile, "%s", `
public_endpoint: https://public.127.0.0.1.nip.io
private_endpoint: https://internal.127.0.0.1.nip.io
port: 8000
key_file: /some/path
cert_file: /some/path
app_stash_config:
  minimum_size: 64K
  maximum_size: 60K
`+
					dummyBlobstoreConfigs)
				_, e := LoadConfig(configFile.Name())
				Expect(e).To(MatchError(ContainSubstring("app_stash_config.maximum_size must be greater than app_stash_config.minimum_size")))
			})

		})
	})

	It("returns an error when blobstores are not configured", func() {
		fmt.Fprintf(configFile, "%s", `
privatebuildpacks:
droplets:
packages:
app_stash:
logging:
  file: /tmp/bits-service.log
  syslog: vcap.bits-service
  level: debug
public_endpoint: https://public.127.0.0.1.nip.io
private_endpoint: https://internal.127.0.0.1.nip.io
secret: geheim
port: 8000
key_file: /some/path
cert_file: /some/path
`)
		_, e := LoadConfig(configFile.Name())

		Expect(e).To(MatchError(SatisfyAll(
			ContainSubstring("Blobstore type '' for droplets is invalid."),
			ContainSubstring("Blobstore type '' for packages is invalid."),
			ContainSubstring("Blobstore type '' for buildpacks is invalid."),
			ContainSubstring("Blobstore type '' for app_stash is invalid."),
		)))

	})

})
