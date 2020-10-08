package oci_registry_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	inmemory_blobstore "github.com/cloudfoundry-incubator/bits-service/blobstores/inmemory"
	"github.com/cloudfoundry-incubator/bits-service/oci_registry"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/urfave/negroni"

	"github.com/cloudfoundry-incubator/bits-service/middlewares"
	"github.com/cloudfoundry-incubator/bits-service/routes"
)

var _ = Describe("Registry", func() {
	var (
		fakeServer        *httptest.Server
		serverURL         string
		rootFSBlobstore   *inmemory_blobstore.Blobstore
		dropletBlobstore  *inmemory_blobstore.Blobstore
		digestLookupStore *inmemory_blobstore.Blobstore
		imageManager      *oci_registry.BitsImageManager
		droplet           []byte
		client            *http.Client
	)

	doGetRequest := func(endpoint string) (*http.Response, error) {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s%s", serverURL, endpoint), nil)
		req.SetBasicAuth("test-user", "test-password")
		Expect(err).ToNot(HaveOccurred())

		return client.Do(req)
	}

	BeforeSuite(func() {
		var e error
		droplet, e = ioutil.ReadFile("assets/example_droplet")
		Expect(e).NotTo(HaveOccurred())
		rootFSBlobstore = inmemory_blobstore.NewBlobstoreWithEntries(map[string][]byte{"assets/eirinifs.tar": []byte("the-rootfs-blob")})
		dropletBlobstore = inmemory_blobstore.NewBlobstore()
		digestLookupStore = inmemory_blobstore.NewBlobstore()
		imageManager = oci_registry.NewBitsImageManager(rootFSBlobstore, dropletBlobstore, digestLookupStore)
		router := mux.NewRouter()

		routes.AddImageHandler(router,
			&oci_registry.ImageHandler{ImageManager: imageManager},
			middlewares.NewBasicAuthMiddleWare(middlewares.Credential{Username: "test-user", Password: "test-password"}),
		)
		fakeServer = httptest.NewServer(negroni.New(
			// middlewares.NewZapLoggerMiddleware(logger.Log),
			&middlewares.PanicMiddleware{},
			negroni.Wrap(router)))
		serverURL = fakeServer.URL

		client = &http.Client{}
	})

	AfterSuite(func() {
		fakeServer.Close()
	})

	BeforeEach(func() {
		dropletBlobstore.Entries["the-droplet-guid/the-droplet-hash"] = droplet
	})

	AfterEach(func() {
		dropletBlobstore.Entries = make(map[string][]byte)
		digestLookupStore.Entries = make(map[string][]byte)
	})

	It("Serves the /v2 endpoint so that the client skips authentication", func() {
		res, e := doGetRequest("/v2/")
		Expect(res.StatusCode, e).To(Equal(200))
	})

	Describe("pull image", func() {
		It("should serve the GET image manifest endpoint", func() {

			res, e := doGetRequest("/v2/cloudfoundry/the-droplet-guid/manifests/the-droplet-hash")
			Expect(res.StatusCode, e).To(Equal(http.StatusOK))
			Expect(ioutil.ReadAll(res.Body)).To(MatchJSON(`{
				"schemaVersion": 2,
				"mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
				"manifests": [
				  {
					"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
					"digest": "sha256:a633dfab15a2f56b3e5d05971fb5e17e91cbec5fbdc192c6873e059da75c387b",
					"size": 582,
					"platform": {
					  "architecture": "amd64",
					  "os": "linux"
					}
				  }
				]
			  }`))

			res, e = doGetRequest("/v2/irrelevant-image-name/manifests/sha256:a633dfab15a2f56b3e5d05971fb5e17e91cbec5fbdc192c6873e059da75c387b")
			Expect(res.StatusCode, e).To(Equal(http.StatusOK))
			Expect(ioutil.ReadAll(res.Body)).To(MatchJSON(`{
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"schemaVersion": 2,
				"config": {
					"mediaType": "application/vnd.docker.container.image.v1+json",
					"digest": "sha256:63460d0ba1dd1ec2ad8889a8dc46382209c0929c8f6421e51de715648fd337c3",
					"size": 219
				},
				"layers": [
					{
						"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
						"digest": "sha256:56ca430559f451494a0e97ff4989ebe28b5d61041f1d7cf8f244acc76974df20",
						"size": 15
					},
					{
						"mediaType": "application/vnd.docker.image.rootfs.diff.tar",
						"digest": "sha256:207cb7b868154dcc33cead383ada32531a856d4c79c307f118893619a6dfe60b",
						"size": 86134392
					}
				]
			}`))

			res, e = doGetRequest("/v2/irrelevant-image-name/blobs/sha256:63460d0ba1dd1ec2ad8889a8dc46382209c0929c8f6421e51de715648fd337c3")
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(res.Body)).To(MatchJSON(`{
				"config": {
				  "user": "2000:2000"
				},
				"rootfs": {
				  "diff_ids": [
					"sha256:56ca430559f451494a0e97ff4989ebe28b5d61041f1d7cf8f244acc76974df20",
					"sha256:207cb7b868154dcc33cead383ada32531a856d4c79c307f118893619a6dfe60b"
				  ],
				  "type": "layers"
				}
			  }`))

			res, e = doGetRequest("/v2/irrelevant-image-name/blobs/sha256:56ca430559f451494a0e97ff4989ebe28b5d61041f1d7cf8f244acc76974df20")
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(res.Body)).To(Equal([]byte("the-rootfs-blob")))

			res, e = doGetRequest("/v2/irrelevant-image-name/blobs/sha256:207cb7b868154dcc33cead383ada32531a856d4c79c307f118893619a6dfe60b")
			Expect(e).NotTo(HaveOccurred())
			Expect(res.StatusCode).To(Equal(http.StatusOK))

			ociDroplet, e := ioutil.ReadAll(res.Body)
			Expect(e).NotTo(HaveOccurred())
			_ = ociDroplet
			r := tar.NewReader(bytes.NewReader(ociDroplet))
			for {
				header, e := r.Next()
				if e == io.EOF {
					break
				}
				Expect(e).NotTo(HaveOccurred())
				Expect(header.Name).To(HavePrefix("/home/vcap"))
				Expect(header.Mode).To(Equal(int64(0777)))
				Expect(header.Uid).To(Equal(2000))
				Expect(header.Gid).To(Equal(2000))
			}
		})

		It("returns StatusNotFound when droplet does not exist", func() {
			res, e := doGetRequest("/v2/image/name/manifests/non-existing-droplet-guid")

			Expect(res.StatusCode, e).To(Equal(http.StatusNotFound))
		})

		It("returns StatusNotFound when layer cannot be found", func() {
			res, e := doGetRequest("/v2/the-image/blobs/not-existent")

			Expect(res.StatusCode, e).To(Equal(http.StatusNotFound))
		})

		Context("image names have multiple paths or special chars", func() {
			It("supports / in the name path parameter", func() {
				res, e := doGetRequest("/v2/cloudfoundry/the-droplet-guid/manifests/the-droplet-hash")

				Expect(res.StatusCode, e).To(Equal(http.StatusOK))
			})

			It("does not allow special characters in the name path parameter", func() {
				res, e := doGetRequest("/v2/image/tag@/v/!22/name/manifests/the-droplet-guid")

				Expect(res.StatusCode, e).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("ImageManager", func() {
		Describe("DeleteArtifacts", func() {
			var origMaxDepth uint

			BeforeEach(func() {
				origMaxDepth = format.MaxDepth
				format.MaxDepth = 0 // setting it to 0 to avoid huge output by the expectations in this spec
			})

			AfterEach(func() {
				format.MaxDepth = origMaxDepth
			})

			It("deletes all OCI artifacts that were created during an image pull", func() {
				Expect(digestLookupStore.Entries).To(BeEmpty()) // Explicitly stating pre-condition

				_, e := doGetRequest("/v2/cloudfoundry/the-droplet-guid/manifests/the-droplet-hash")
				Expect(e).NotTo(HaveOccurred())

				Expect(digestLookupStore.Entries).To(HaveLen(4)) // manifest index + manifest + config + droplet blob = 4 (rootfs blob is in rootfs blobstore)

				e = imageManager.DeleteArtifacts("the-droplet-guid", "the-droplet-hash")
				Expect(e).NotTo(HaveOccurred(), "%+v", e)

				Expect(digestLookupStore.Entries).To(BeEmpty())
			})
		})
	})
})
