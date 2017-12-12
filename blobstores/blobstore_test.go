package blobstores_test

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/onsi/ginkgo"

	. "github.com/onsi/ginkgo"

	"github.com/onsi/gomega"

	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo"
	inmemory "github.com/petergtz/bitsgo/blobstores/inmemory"
	"github.com/petergtz/bitsgo/blobstores/local"
	"github.com/petergtz/bitsgo/config"
	"os"
)

func TestInMemoryBlobstore(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "InMemory Blobstore")
}

var _ = Describe("Blobstore", func() {
	var blobstore bitsgo.Blobstore

	itCanBeModifiedByItsMethods := func() {
		It("can be modified by its methods", func() {
			Expect(blobstore.Exists("/some/path")).To(BeFalse())

			redirectLocation, e := blobstore.HeadOrRedirectAsGet("/some/path")
			Expect(redirectLocation).To(BeEmpty())
			Expect(e).To(BeAssignableToTypeOf(bitsgo.NewNotFoundError()))

			Expect(blobstore.Put("/some/path", strings.NewReader("some string"))).To(Succeed())

			Expect(blobstore.Exists("/some/path")).To(BeTrue())

			Expect(blobstore.HeadOrRedirectAsGet("/some/path")).To(BeEmpty())

			body, redirectLocation, e := blobstore.GetOrRedirect("/some/path")
			Expect(redirectLocation, e).To(BeEmpty())
			Expect(ioutil.ReadAll(body)).To(MatchRegexp("some string"))

			Expect(blobstore.Copy("/some/path", "/some/other/path")).To(Succeed())
			Expect(blobstore.Copy("/some/other/path", "/some/yet/other/path")).To(Succeed())
			Expect(blobstore.Copy("/some/other/path", "/yet/some/other/path")).To(Succeed())
			Expect(blobstore.Copy("/yet/some/other/path", "/yet/some/other/path")).To(Succeed())

			body, redirectLocation, e = blobstore.GetOrRedirect("/some/other/path")
			Expect(redirectLocation, e).To(BeEmpty())
			Expect(ioutil.ReadAll(body)).To(MatchRegexp("some string"))

			Expect(blobstore.Delete("/some/path")).To(Succeed())

			Expect(blobstore.Exists("/some/path")).To(BeFalse())

			Expect(blobstore.Exists("/some/other/path")).To(BeTrue())

			redirectLocation, e = blobstore.HeadOrRedirectAsGet("/some/path")
			Expect(redirectLocation).To(BeEmpty())
			Expect(e).To(BeAssignableToTypeOf(bitsgo.NewNotFoundError()))

			Expect(blobstore.DeleteDir("/some")).To(Succeed())
			Expect(blobstore.Exists("/some/other/path")).To(BeFalse())
			Expect(blobstore.Exists("/some/yet/other/path")).To(BeFalse())
			Expect(blobstore.Exists("/yet/some/other/path")).To(BeTrue())

			Expect(blobstore.DeleteDir("")).To(Succeed())
			Expect(blobstore.Exists("/yet/some/other/path")).To(BeFalse())
		})
	}

	Describe("Local", func() {
		var tempDirname string

		BeforeEach(func() {
			var e error
			tempDirname, e = ioutil.TempDir("", "bitsgo")
			Expect(e).NotTo(HaveOccurred())

			blobstore = local.NewBlobstore(config.LocalBlobstoreConfig{PathPrefix: tempDirname})
		})
		AfterEach(func() { os.RemoveAll(tempDirname) })

		itCanBeModifiedByItsMethods()
	})

	Describe("In-memory", func() {
		BeforeEach(func() { blobstore = inmemory.NewBlobstore() })

		itCanBeModifiedByItsMethods()
	})
})
