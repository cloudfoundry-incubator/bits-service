package config_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	. "github.com/petergtz/bitsgo/config"
)

func TestConfig(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Config")
}

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
public_endpoint: public.127.0.0.1.xip.io
private_endpoint: internal.127.0.0.1.xip.io
secret: geheim
port: 8000
`)
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

})
