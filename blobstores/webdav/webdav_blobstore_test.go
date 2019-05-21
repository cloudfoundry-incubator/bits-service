package webdav_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/bits-service/blobstores/webdav"
	"github.com/cloudfoundry-incubator/bits-service/config"
)

var _ = Describe("WebdavBlobstore", func() {
	Describe("DeleteDir", func() {
		var (
			webdavBlobstore *Blobstore
			testServer      *httptest.Server
		)

		BeforeEach(func() {
			testServer = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				Expect(req.URL).ToNot(HaveSuffix("//"))
				Expect(req.URL).To(HaveSuffix("/"))
			}))
			webdavBlobstore = NewBlobstoreWithHttpClient(config.WebdavBlobstoreConfig{
				PrivateEndpoint: testServer.URL,
				PublicEndpoint:  testServer.URL,
			}, &http.Client{})
		})

		AfterEach(func() { testServer.Close() })

		It("appends slash to url if needed", func() {
			webdavBlobstore.DeleteDir("path/without/slash/suffix")
		})

		It("does not append a slash when there is already a slash at the end", func() {
			webdavBlobstore.DeleteDir("path/with/single/slash/suffix/")
		})
	})
})
