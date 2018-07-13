package routes_test

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"archive/zip"

	"io/ioutil"

	"github.com/gorilla/mux"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/blobstores/decorator"
	"github.com/petergtz/bitsgo/blobstores/inmemory"
	"github.com/petergtz/bitsgo/httputil"
	. "github.com/petergtz/bitsgo/routes"
	"github.com/petergtz/bitsgo/statsd"
	. "github.com/petergtz/bitsgo/testutil"
	"github.com/petergtz/pegomock"
	. "github.com/petergtz/pegomock"
)

func TestRoutes(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	pegomock.RegisterMockFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Routes")
}

// TODO this test does more than just routes. Maybe it should be moved to the cmd package instead.
var _ = Describe("routes", func() {
	BeforeSuite(func() {
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
	})

	var (
		blobstoreEntries  map[string][]byte
		blobstore         *inmemory_blobstore.Blobstore
		appstashBlobstore *inmemory_blobstore.Blobstore
		router            *mux.Router
		responseWriter    *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		blobstoreEntries = make(map[string][]byte)
		blobstore = inmemory_blobstore.NewBlobstoreWithEntries(blobstoreEntries)
		appstashBlobstore = inmemory_blobstore.NewBlobstore()
		router = mux.NewRouter()
		responseWriter = httptest.NewRecorder()
	})

	ItSupportsMethodsGetPutDeleteFor := func(path string, resourceType string, blobstoreKey string) {
		Context("Method GET", func() {
			It("returns StatusNotFound when blobstore returns NotFoundError", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", path, nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
			})

			It("returns StatusOK and fills body with contents from file located at the partitioned path", func() {
				blobstoreEntries[blobstoreKey] = []byte("thecontent")

				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", path, nil))

				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusOK),
					Equal("thecontent")))
			})
		})

		Context("Method HEAD", func() {
			It("returns StatusNotFound when blobstore returns NotFoundError", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("HEAD", path, nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
			})

			It("returns StatusOK and leaves body empty", func() {
				blobstoreEntries[blobstoreKey] = []byte("thecontent")

				router.ServeHTTP(responseWriter, httptest.NewRequest("HEAD", path, nil))

				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusOK),
					Equal("")))
			})
		})

		Context("Method PUT", func() {
			It("returns StatusBadRequest when "+resourceType+" form file field is missing in request body", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("PUT", path, nil))

				Expect(responseWriter.Code).To(Equal(http.StatusBadRequest))
			})

			It("returns StatusOK and an empty body, and forwards the file reader to the blobstore", func() {
				router.ServeHTTP(responseWriter, newHttpTestPutRequest(path, map[string]map[string]io.Reader{
					resourceType: map[string]io.Reader{"somefilename": CreateZip(map[string]string{})},
				}))
				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusCreated),
					// TODO: Use a proper json comparison.
					SatisfyAll(
						MatchRegexp("{.*}"),
						MatchRegexp(`.*"guid" *: *"[A-Za-z0-9/]+".*`),
						MatchRegexp(`.*"state" *: *"READY".*`),
						MatchRegexp(`.*"type" *: *"bits".*`),
						MatchRegexp(`.*"created_at" *:.*`),
						MatchRegexp(`.*"sha1" *: *"[a-z0-9]{40}".*`),
						MatchRegexp(`.*"sha256" *: *"[a-z0-9]{64}".*`),
					)))

				Expect(blobstoreEntries).To(HaveKeyWithValue(blobstoreKey, CreateZip(map[string]string{}).Bytes()))
			})
		})

		Context("Method DELETE", func() {
			It("returns StatusNotFound when blobstore returns NotFoundError", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("DELETE", path, nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
			})

			It("returns StatusOK", func() {
				blobstoreEntries[blobstoreKey] = []byte("thecontent")

				router.ServeHTTP(responseWriter, httptest.NewRequest("DELETE", path, nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNoContent))
			})
		})
	}

	Describe("/packages/{guid}", func() {
		BeforeEach(func() {
			SetUpPackageRoutes(
				router,
				bitsgo.NewResourceHandler(decorator.ForBlobstoreWithPathPartitioning(blobstore), appstashBlobstore, "package", statsd.NewMetricsService(), 0))
		})
		ItSupportsMethodsGetPutDeleteFor("/packages/theguid", "package", "th/eg/theguid")
	})

	Describe("/droplets/{guid}", func() {
		BeforeEach(func() {
			SetUpDropletRoutes(
				router,
				bitsgo.NewResourceHandler(decorator.ForBlobstoreWithPathPartitioning(blobstore), appstashBlobstore, "droplet", statsd.NewMetricsService(), 0))
		})

		Context("With digest in URL (/droplets/{guid}/{checksum})", func() {
			ItSupportsMethodsGetPutDeleteFor("/droplets/theguid/checksum", "droplet", "th/eg/theguid/checksum")
		})

		Context("With digest in Header (/droplets/{guid}/{digest}; Header: Digest: sha256={checksum})", func() {
			It("reads the digest from the header", func() {
				r, e := http.NewRequest("PUT", "/droplets/theguid", strings.NewReader("My test string"))
				Expect(e).NotTo(HaveOccurred())
				r.Header.Set("Digest", "sha256=checksum")

				router.ServeHTTP(responseWriter, r)

				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusCreated),
					// TODO: Use a proper json comparison.
					SatisfyAll(
						MatchRegexp("{.*}"),
						MatchRegexp(`.*"guid" *: *"[A-Za-z0-9/]*".*`),
						MatchRegexp(`.*"state" *: *"READY".*`),
						MatchRegexp(`.*"type" *: *"bits".*`),
						MatchRegexp(`.*"created_at" *:.*`),
					)))

				Expect(blobstoreEntries).To(HaveKeyWithValue("th/eg/theguid/checksum", []byte("My test string")))
			})
		})
	})

	Describe("/buildpacks/{guid}", func() {
		BeforeEach(func() {
			SetUpBuildpackRoutes(
				router,
				bitsgo.NewResourceHandler(decorator.ForBlobstoreWithPathPartitioning(blobstore), appstashBlobstore, "buildpack", statsd.NewMetricsService(), 0))
		})
		ItSupportsMethodsGetPutDeleteFor("/buildpacks/theguid", "buildpack", "th/eg/theguid")
	})

	Describe("/buildpack_cache/entries/{app_guid}/{stack_name}", func() {
		BeforeEach(func() {
			SetUpBuildpackCacheRoutes(
				router,
				bitsgo.NewResourceHandler(decorator.ForBlobstoreWithPathPartitioning(decorator.ForBlobstoreWithPathPrefixing(blobstore, "buildpack_cache/")), appstashBlobstore, "buildpack_cache", statsd.NewMetricsService(), 0))
		})
		Context("Method GET", func() {
			It("returns StatusNotFound when blobstore returns NotFoundError", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", "/buildpack_cache/entries/theguid/thestackname", nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
			})

			It("returns StatusOK and fills body with contents from file located at the partitioned path", func() {
				blobstoreEntries["buildpack_cache/th/eg/theguid/thestackname"] = []byte("thecontent")

				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", "/buildpack_cache/entries/theguid/thestackname", nil))

				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusOK),
					Equal("thecontent")))
			})
		})
	})

	Describe("/app_stash", func() {
		BeforeEach(func() {
			SetUpAppStashRoutes(router, bitsgo.NewAppStashHandler(blobstore, 0))
		})

		Describe("/app_stash/entries", func() {
			It("Unzips file and copies bits to blobstore", func() {
				zipFile, e := os.Open("test_data/test_archive.zip")
				Expect(e).NotTo(HaveOccurred())
				defer zipFile.Close()

				router.ServeHTTP(responseWriter, newHttpTestPostRequest("/app_stash/entries", map[string]map[string]io.Reader{
					"application": map[string]io.Reader{"application": zipFile},
				}))

				Expect(responseWriter.Code).To(Equal(http.StatusCreated), responseWriter.Body.String())
				Expect(responseWriter.Body.String()).To(MatchJSON(
					`[
						{"sha1":"27cc6f77ee63df90ab3285f9d5fc4ebcb2448c12","fn":"test folder/three","size":3,"mode":"664"},
						{"sha1":"971555ab39d1dfe8dff8b78c2b20e85e01c06595","fn":"one","size":3,"mode":"664"},
						{"sha1":"bbd33de01c17b165b4ce00308e8a19a942023ab8","fn":"two","size":3,"mode":"664"}
					]`))
				Expect(blobstoreEntries).To(HaveKeyWithValue("971555ab39d1dfe8dff8b78c2b20e85e01c06595", []byte("1\n\n")))
				Expect(blobstoreEntries).To(HaveKeyWithValue("bbd33de01c17b165b4ce00308e8a19a942023ab8", []byte("2\n\n")))
				Expect(blobstoreEntries).To(HaveKeyWithValue("27cc6f77ee63df90ab3285f9d5fc4ebcb2448c12", []byte("3\n\n")))
			})
		})

		Describe("/app_stash/matches", func() {
			It("returns StatusUnprocessableEntity when body is invalid", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest(
					"POST", "/app_stash/matches", strings.NewReader("some invalid format")))

				Expect(responseWriter.Code).To(Equal(http.StatusUnprocessableEntity), responseWriter.Body.String())
			})

			It("returns StatusOK and matching fingerprints when body is valid", func() {
				blobstoreEntries["abc"] = []byte("not relevant")

				router.ServeHTTP(responseWriter, httptest.NewRequest(
					"POST", "/app_stash/matches", strings.NewReader(`[
						{"sha1":"abc","size":123, "fn":"some-file", "mode":"644"},
						{"sha1":"def","size":456, "fn":"some-other-file", "mode":"644"}
					]`)))

				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusOK),
					MatchJSON(`[{"sha1":"abc","size":123, "fn":"some-file", "mode":"644"}]`)))
			})

			It("returns StatusOK and an empty list when none of the entries match", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest(
					"POST", "/app_stash/matches", strings.NewReader(`[{"sha1":"abc","size":123}, {"sha1":"def","size":456}]`)))

				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusOK),
					Equal("[]")))
			})
		})

		Describe("/app_stash/bundles", func() {
			It("returns StatusUnprocessableEntity when body is invalid", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest(
					"POST", "/app_stash/bundles", strings.NewReader("some invalid format")))

				Expect(responseWriter.Code).To(Equal(http.StatusUnprocessableEntity), responseWriter.Body.String())
			})

			It("downloads files identified by sha1s from blobstore, zips them and returns zip", func() {
				blobstoreEntries["sha1xyz"] = []byte("some content")
				blobstoreEntries["sha1abc"] = []byte("some more content")

				router.ServeHTTP(responseWriter, httptest.NewRequest(
					"POST", "/app_stash/bundles", strings.NewReader("[{\"sha1\":\"sha1xyz\", \"fn\":\"filename1\"}, {\"sha1\":\"sha1abc\", \"fn\":\"filename2\"}]")))

				Expect(responseWriter.Code).To(Equal(http.StatusOK))
				// TODO: this should also verify filemodes of the newly created files
				Expect(zipContentsOf(responseWriter.Body)).To(SatisfyAll(
					HaveKeyWithValue("filename1", []byte("some content")),
					HaveKeyWithValue("filename2", []byte("some more content"))))
			})
		})
	})
})

func zipContentsOf(buffer *bytes.Buffer) map[string][]byte {
	zipReader, e := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	Expect(e).NotTo(HaveOccurred())

	zipContents := make(map[string][]byte)
	for _, zipFileEntry := range zipReader.File {
		reader, e := zipFileEntry.Open()
		Expect(e).NotTo(HaveOccurred())
		zipContents[zipFileEntry.Name] = MustReadAll(reader)
	}
	return zipContents
}

func MustReadAll(reader io.Reader) []byte {
	content, e := ioutil.ReadAll(reader)
	Expect(e).NotTo(HaveOccurred())
	return content
}

// TODO: either remove or add tests that use this function, e.g. tests where blobstore return an error
func writeStatusCodeAndBody(statusCode int, body string) func([]Param) ReturnValues {
	return func(params []Param) ReturnValues {
		for _, param := range params {
			if responseWriter, ok := param.(http.ResponseWriter); ok {
				responseWriter.WriteHeader(statusCode)
				responseWriter.Write([]byte(body))
				return nil
			}
		}
		panic("Unexpected: no ResponseWriter in parameter list.")
	}
}

func newHttpTestPutRequest(path string, formFiles map[string]map[string]io.Reader) *http.Request {
	return newHttpTestRequest("PUT", path, formFiles)
}

func newHttpTestPostRequest(path string, formFiles map[string]map[string]io.Reader) *http.Request {
	return newHttpTestRequest("POST", path, formFiles)
}

func newHttpTestRequest(method string, path string, formFiles map[string]map[string]io.Reader) *http.Request {
	bodyBuf := &bytes.Buffer{}
	contentType, e := httputil.AddFormFileTo(bodyBuf, formFiles)
	Expect(e).NotTo(HaveOccurred())
	request := httptest.NewRequest(method, path, bodyBuf)
	request.Header.Add("Content-Type", contentType)
	return request
}

func AnyReadSeeker() io.ReadSeeker {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*io.ReadSeeker)(nil)).Elem()))
	return nil
}
