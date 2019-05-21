package webdav_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/bits-service/blobstores/webdav"
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

			webdavBlobstore = &Blobstore{
				WebdavPrivateEndpoint: testServer.URL,
				WebdavPublicEndpoint:  testServer.URL,
				HttpClient:            &http.Client{},
			}
		})

		AfterEach(func() {
			defer func() { testServer.Close() }()
		})

		It("appends slash to url if needed", func() {
			webdavBlobstore.DeleteDir("path/without/slash/suffix")
		})

		It("does not append a slash when there is already a slash at the end", func() {
			webdavBlobstore.DeleteDir("path/with/single/slash/suffix/")
		})
	})
})
