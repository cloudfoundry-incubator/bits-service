package webdav_test

import (
	//"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/bits-service/blobstores/webdav"
)

var _ = Describe("webdav DeleteDirectory", func() {
	It("appends a slash if there is none", func() {
		prefix := AppendsSuffixIfNeeded("/some/path")
		Expect(prefix).ToNot(HaveSuffix("//"))
		Expect(prefix).To(Equal("/some/path/"))
	})
	It("does not append a slash if there already is a slash at the end", func() {
		prefix := AppendsSuffixIfNeeded("/some/path/")
		Expect(prefix).ToNot(HaveSuffix("//"))
		Expect(prefix).To(Equal("/some/path/"))
	})
	Context("webdav integration test", func() {
		var webdavBlobstore *Blobstore
		BeforeEach(func() {
			testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				Expect(req.URL).ToNot(HaveSuffix("//"))
				Expect(req.URL).To(HaveSuffix("/"))
			}))

			defer func() { testServer.Close() }()

			webdavBlobstore = &Blobstore{
				WebdavPrivateEndpoint: testServer.URL,
				WebdavPublicEndpoint:  testServer.URL,
				HttpClient:            &http.Client{},
				WebdavUsername:        "foo",
				WebdavPassword:        "bar",
			}
		})
		It("appends slash to url if needed", func() {
			webdavBlobstore.DeleteDir("/path/without/slash/suffix")
		})
		It("does not append a slash when there is already a slas at the end", func() {
			webdavBlobstore.DeleteDir("/path/with/single/slash/suffix/")
		})
	})
})
