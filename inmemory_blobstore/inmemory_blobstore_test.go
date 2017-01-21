package inmemory_blobstore_test

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/onsi/ginkgo"
	. "github.com/petergtz/bitsgo/inmemory_blobstore"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo/routes"
)

func TestInMemoryBlobstore(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "InMemory Blobstore")
}

var _ = Describe("Blobstore", func() {
	It("can be modified by its methods", func() {
		blobstore := NewInMemoryBlobstore()
		Expect(blobstore.Exists("/some/path")).To(BeFalse())

		redirectLocation, e := blobstore.Head("/some/path")
		Expect(redirectLocation).To(BeEmpty())
		Expect(e).To(BeAssignableToTypeOf(routes.NewNotFoundError()))

		Expect(blobstore.Put("/some/path", strings.NewReader("some string"))).To(BeEmpty())

		Expect(blobstore.Exists("/some/path")).To(BeTrue())

		Expect(blobstore.Head("/some/path")).To(BeEmpty())

		body, redirectLocation, e := blobstore.Get("/some/path")
		Expect(redirectLocation, e).To(BeEmpty())
		Expect(ioutil.ReadAll(body)).To(MatchRegexp("some string"))

		Expect(blobstore.Copy("/some/path", "/some/other/path")).To(BeEmpty())
		Expect(blobstore.Copy("/some/other/path", "/some/yet/other/path")).To(BeEmpty())
		Expect(blobstore.Copy("/some/other/path", "/yet/some/other/path")).To(BeEmpty())
		Expect(blobstore.Copy("/yet/some/other/path", "/yet/some/other/path")).To(BeEmpty())

		body, redirectLocation, e = blobstore.Get("/some/other/path")
		Expect(redirectLocation, e).To(BeEmpty())
		Expect(ioutil.ReadAll(body)).To(MatchRegexp("some string"))

		Expect(blobstore.Delete("/some/path")).To(Succeed())

		Expect(blobstore.Exists("/some/path")).To(BeFalse())

		Expect(blobstore.Exists("/some/other/path")).To(BeTrue())

		redirectLocation, e = blobstore.Head("/some/path")
		Expect(redirectLocation).To(BeEmpty())
		Expect(e).To(BeAssignableToTypeOf(routes.NewNotFoundError()))

		Expect(blobstore.DeletePrefix("/some")).To(Succeed())
		Expect(blobstore.Exists("/some/other/path")).To(BeFalse())
		Expect(blobstore.Exists("/some/yet/other/path")).To(BeFalse())
		Expect(blobstore.Exists("/yet/some/other/path")).To(BeTrue())

		Expect(blobstore.DeletePrefix("")).To(Succeed())
		Expect(blobstore.Exists("/yet/some/other/path")).To(BeFalse())
	})
})
