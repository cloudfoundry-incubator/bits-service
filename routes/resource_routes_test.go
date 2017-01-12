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
	"github.com/petergtz/bitsgo/httputil"
	"github.com/petergtz/bitsgo/inmemory_blobstore"
	. "github.com/petergtz/bitsgo/routes"
	. "github.com/petergtz/bitsgo/testutil"
	"github.com/petergtz/pegomock"
	. "github.com/petergtz/pegomock"
)

//go:generate pegomock generate --use-experimental-model-gen --package routes_test Blobstore

func TestRoutes(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	pegomock.RegisterMockFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Routes")
}

var _ = Describe("routes", func() {
	BeforeSuite(func() {
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
	})

	var (
		blobstoreEntries map[string][]byte
		blobstore        Blobstore
		router           *mux.Router
		responseWriter   *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		blobstoreEntries = make(map[string][]byte)
		blobstore = inmemory_blobstore.NewInMemoryBlobstoreWithEntries(blobstoreEntries)
		router = mux.NewRouter()
		responseWriter = httptest.NewRecorder()
	})

	ItSupportsMethodsGetPutDeleteFor := func(routeName string, resourceType string) {
		Context("Method GET", func() {
			It("returns StatusNotFound when blobstore returns NotFoundError", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", "/"+routeName+"/theguid", nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
			})

			It("returns StatusOK and fills body with contents from file located at the partitioned path", func() {
				blobstoreEntries["th/eg/theguid"] = []byte("thecontent")

				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", "/"+routeName+"/theguid", nil))

				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusOK),
					Equal("thecontent")))
			})
		})

		Context("Method HEAD", func() {
			It("returns StatusNotFound when blobstore returns NotFoundError", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("HEAD", "/"+routeName+"/theguid", nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
			})

			It("returns StatusOK and leaves body empty", func() {
				blobstoreEntries["th/eg/theguid"] = []byte("thecontent")

				router.ServeHTTP(responseWriter, httptest.NewRequest("HEAD", "/"+routeName+"/theguid", nil))

				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusOK),
					Equal("")))
			})
		})

		Context("Method PUT", func() {
			It("returns StatusBadRequest when "+resourceType+" form file field is missing in request body", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("PUT", "/"+routeName+"/theguid", nil))

				Expect(responseWriter.Code).To(Equal(http.StatusBadRequest))
			})

			It("returns StatusOK and an empty body, and forwards the file reader to the blobstore", func() {
				router.ServeHTTP(responseWriter, newHttpTestPutRequest("/"+routeName+"/theguid", map[string]map[string]io.Reader{
					resourceType: map[string]io.Reader{"somefilename": strings.NewReader("My test string")},
				}))

				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusCreated),
					BeEmpty()))

				Expect(blobstoreEntries).To(HaveKeyWithValue("th/eg/theguid", []byte("My test string")))
			})
		})

		Context("Method DELETE", func() {
			It("returns StatusNotFound when blobstore returns NotFoundError", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest("DELETE", "/"+routeName+"/theguid", nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
			})

			It("returns StatusOK", func() {
				blobstoreEntries["th/eg/theguid"] = []byte("thecontent")

				router.ServeHTTP(responseWriter, httptest.NewRequest("DELETE", "/"+routeName+"/theguid", nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNoContent))
			})
		})
	}

	Describe("/packages/{guid}", func() {
		BeforeEach(func() { SetUpPackageRoutes(router, DecorateWithPartitioningPathBlobstore(blobstore)) })
		ItSupportsMethodsGetPutDeleteFor("packages", "package")
	})

	Describe("/droplets/{guid}", func() {
		BeforeEach(func() { SetUpDropletRoutes(router, DecorateWithPartitioningPathBlobstore(blobstore)) })
		ItSupportsMethodsGetPutDeleteFor("droplets", "droplet")
	})

	Describe("/buildpacks/{guid}", func() {
		BeforeEach(func() { SetUpBuildpackRoutes(router, DecorateWithPartitioningPathBlobstore(blobstore)) })
		ItSupportsMethodsGetPutDeleteFor("buildpacks", "buildpack")
	})

	Describe("/buildpack_cache/entries/{app_guid}/{stack_name}", func() {
		BeforeEach(func() {
			SetUpBuildpackCacheRoutes(router, DecorateWithPartitioningPathBlobstore(DecorateWithPrefixingPathBlobstore(blobstore, "buildpack_cache/")))
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
			SetUpAppStashRoutes(router, blobstore)
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
				Expect(responseWriter.Body.String()).To(ContainSubstring(
					`{"sha1":"971555ab39d1dfe8dff8b78c2b20e85e01c06595","fn":"one","mode":"664"}`))
				Expect(responseWriter.Body.String()).To(ContainSubstring(
					`{"sha1":"bbd33de01c17b165b4ce00308e8a19a942023ab8","fn":"two","mode":"664"}`))
				Expect(responseWriter.Body.String()).To(ContainSubstring(
					`{"sha1":"27cc6f77ee63df90ab3285f9d5fc4ebcb2448c12","fn":"test folder/three","mode":"664"}`))
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
					"POST", "/app_stash/matches", strings.NewReader(`[{"sha1":"abc","size":123}, {"sha1":"def","size":456}]`)))

				Expect(*responseWriter).To(HaveStatusCodeAndBody(
					Equal(http.StatusOK),
					Equal("[{\"sha1\":\"abc\"}]")))
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
	header, e := httputil.AddFormFileTo(bodyBuf, formFiles)
	Expect(e).NotTo(HaveOccurred())
	request := httptest.NewRequest(method, path, bodyBuf)
	httputil.AddHeaderTo(request, header)
	return request
}

func AnyReadSeeker() io.ReadSeeker {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*io.ReadSeeker)(nil)).Elem()))
	return nil
}
