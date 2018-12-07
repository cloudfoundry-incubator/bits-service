package oci_registry_test

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry-incubator/bits-service/blobstores/inmemory"
	"github.com/cloudfoundry-incubator/bits-service/oci_registry"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/urfave/negroni"

	"github.com/cloudfoundry-incubator/bits-service"
	"github.com/cloudfoundry-incubator/bits-service/middlewares"
	"github.com/cloudfoundry-incubator/bits-service/routes"
)

var _ = Describe("Registry", func() {
	var (
		fakeServer        *httptest.Server
		serverURL         string
		rootFSBlobstore   bitsgo.NoRedirectBlobstore
		dropletBlobstore  bitsgo.NoRedirectBlobstore
		digestLookupStore bitsgo.NoRedirectBlobstore
		droplet           []byte
	)
	BeforeSuite(func() {
		var e error
		droplet, e = ioutil.ReadFile("assets/example_droplet")
		Expect(e).NotTo(HaveOccurred())
		rootFSBlobstore = inmemory_blobstore.NewBlobstoreWithEntries(map[string][]byte{"assets/eirinifs.tar": []byte("the-rootfs-blob")})
		dropletBlobstore = inmemory_blobstore.NewBlobstoreWithEntries(map[string][]byte{"the-droplet-guid/the-droplet-hash": droplet})
		digestLookupStore = inmemory_blobstore.NewBlobstore()
		imageManager := oci_registry.NewBitsImageManager(rootFSBlobstore, dropletBlobstore, digestLookupStore)
		router := mux.NewRouter()

		routes.AddImageHandler(router, &oci_registry.ImageHandler{
			ImageManager: imageManager,
		})
		fakeServer = httptest.NewServer(negroni.New(
			// middlewares.NewZapLoggerMiddleware(logger.Log),
			&middlewares.PanicMiddleware{},
			negroni.Wrap(router)))
		serverURL = fakeServer.URL
	})

	AfterSuite(func() {
		fakeServer.Close()
	})

	It("Serves the /v2 endpoint so that the client skips authentication", func() {
		res, e := http.Get(serverURL + "/v2/")

		Expect(res.StatusCode, e).To(Equal(200))
	})

	Describe("pull image", func() {
		It("should serve the GET image manifest endpoint", func() {
			res, e := http.Get(serverURL + "/v2/cloudfoundry/the-droplet-guid/manifests/the-droplet-hash")

			Expect(res.StatusCode, e).To(Equal(http.StatusOK))
			Expect(ioutil.ReadAll(res.Body)).To(MatchJSON(`{
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"schemaVersion": 2,
				"config": {
					"mediaType": "application/vnd.docker.container.image.v1+json",
					"digest": "sha256:e45cc3b91fb3e12c17e8a9a3a9d476f516f480756795a2b446f0a129f6bcae3d",
					"size": 214
				},
				"layers": [
					{
						"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
						"digest": "sha256:56ca430559f451494a0e97ff4989ebe28b5d61041f1d7cf8f244acc76974df20",
						"size": 15
					},
					{
						"mediaType": "application/vnd.docker.image.rootfs.diff.tar",
						"digest": "sha256:ccba5ce536c29da80ff2da1c81fc7b9e4d07ab679b6bfb03964432f116d61dd7",
						"size": 86134392
					}
				]
			}`))

			res, e = http.Get(serverURL + "/v2/irrelevant-image-name/blobs/sha256:e45cc3b91fb3e12c17e8a9a3a9d476f516f480756795a2b446f0a129f6bcae3d")
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(res.Body)).To(MatchJSON(`{
				"config": {
				  "user": "vcap"
				},
				"rootfs": {
				  "diff_ids": [
					"sha256:56ca430559f451494a0e97ff4989ebe28b5d61041f1d7cf8f244acc76974df20",
					"sha256:ccba5ce536c29da80ff2da1c81fc7b9e4d07ab679b6bfb03964432f116d61dd7"
				  ],
				  "type": "layers"
				}
			  }`))

			res, e = http.Get(serverURL + "/v2/irrelevant-image-name/blobs/sha256:56ca430559f451494a0e97ff4989ebe28b5d61041f1d7cf8f244acc76974df20")
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(res.Body)).To(Equal([]byte("the-rootfs-blob")))

			res, e = http.Get(serverURL + "/v2/irrelevant-image-name/blobs/sha256:ccba5ce536c29da80ff2da1c81fc7b9e4d07ab679b6bfb03964432f116d61dd7")
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
			}
		})

		It("returns StatusNotFound when droplet does not exist", func() {
			res, e := http.Get(serverURL + "/v2/image/name/manifests/non-existing-droplet-guid")

			Expect(res.StatusCode, e).To(Equal(http.StatusNotFound))
		})

		It("returns StatusNotFound when layer cannot be found", func() {
			res, e := http.Get(serverURL + "/v2/the-image/blobs/not-existent")

			Expect(res.StatusCode, e).To(Equal(http.StatusNotFound))
		})

		Context("image names have multiple paths or special chars", func() {
			It("supports / in the name path parameter", func() {
				res, e := http.Get(serverURL + "/v2/cloudfoundry/the-droplet-guid/manifests/the-droplet-hash")

				Expect(res.StatusCode, e).To(Equal(http.StatusOK))
			})

			It("does not allow special characters in the name path parameter", func() {
				res, e := http.Get(serverURL + "/v2/image/tag@/v/!22/name/manifests/the-droplet-guid")

				Expect(res.StatusCode, e).To(Equal(http.StatusNotFound))
			})
		})
	})
})
