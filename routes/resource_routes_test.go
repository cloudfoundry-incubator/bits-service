package routes_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/petergtz/bitsgo/httputil"
	. "github.com/petergtz/bitsgo/routes"
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
		blobstore      *MockBlobstore
		router         *mux.Router
		responseWriter *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		blobstore = NewMockBlobstore()
		router = mux.NewRouter()
		responseWriter = httptest.NewRecorder()
	})

	ItSupportsMethodsGetPutDeleteFor := func(routeName string, resourceType string) {
		Context("Method GET", func() {
			It("returns StatusNotFound when blobstore returns NotFoundError", func() {
				When(func() { blobstore.Get(AnyString()) }).
					ThenReturn(nil, "", NewNotFoundError())

				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", "/"+routeName+"/theguid", nil))

				Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
			})

			It("returns StatusOK and fills body with contents from file located at the partitioned path", func() {
				When(func() { blobstore.Get("/th/eg/theguid") }).
					ThenReturn(ioutil.NopCloser(strings.NewReader("thecontent")), "", nil)

				router.ServeHTTP(responseWriter, httptest.NewRequest("GET", "/"+routeName+"/theguid", nil))

				Expect(*responseWriter).To(haveStatusCodeAndBody(
					Equal(http.StatusOK),
					Equal("thecontent")))
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

				Expect(*responseWriter).To(haveStatusCodeAndBody(
					Equal(http.StatusCreated),
					BeEmpty()))

				_, fileContent := blobstore.VerifyWasCalledOnce().Put(EqString("/th/eg/theguid"), AnyReadSeeker()).GetCapturedArguments()
				Expect(ioutil.ReadAll(fileContent)).To(MatchRegexp("My test string"))
			})
		})
	}

	Describe("/packages/{guid}", func() {
		BeforeEach(func() { SetUpPackageRoutes(router, blobstore) })
		ItSupportsMethodsGetPutDeleteFor("packages", "package")
	})

	Describe("/droplets/{guid}", func() {
		BeforeEach(func() { SetUpDropletRoutes(router, blobstore) })
		ItSupportsMethodsGetPutDeleteFor("droplets", "droplet")
	})

	Describe("/buildpacks/{guid}", func() {
		BeforeEach(func() { SetUpBuildpackRoutes(router, blobstore) })
		ItSupportsMethodsGetPutDeleteFor("buildpacks", "buildpack")
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

				entries := make(map[string]string)

				When(blobstore.Put(AnyString(), AnyReadSeeker())).Then(func(params []pegomock.Param) pegomock.ReturnValues {
					content, e := ioutil.ReadAll(params[1].(io.ReadSeeker))
					Expect(e).NotTo(HaveOccurred())
					entries[params[0].(string)] = string(content)
					return nil
				})

				router.ServeHTTP(responseWriter, newHttpTestPostRequest("/app_stash/entries", map[string]map[string]io.Reader{
					"application": map[string]io.Reader{"application": zipFile},
				}))

				Expect(responseWriter.Code).To(Equal(http.StatusOK), responseWriter.Body.String())
				Expect(entries).To(HaveKeyWithValue("971555ab39d1dfe8dff8b78c2b20e85e01c06595", "1\n\n"))
				Expect(entries).To(HaveKeyWithValue("bbd33de01c17b165b4ce00308e8a19a942023ab8", "2\n\n"))
				Expect(entries).To(HaveKeyWithValue("27cc6f77ee63df90ab3285f9d5fc4ebcb2448c12", "3\n\n"))
			})
		})

		Describe("/app_stash/matches", func() {
			It("returns StatusUnprocessableEntity when body is invalid", func() {
				router.ServeHTTP(responseWriter, httptest.NewRequest(
					"POST", "/app_stash/matches", strings.NewReader("some invalid format")))

				Expect(responseWriter.Code).To(Equal(http.StatusUnprocessableEntity), responseWriter.Body.String())
			})

			It("returns StatusOK and missing fingerprints when body is valid", func() {
				When(blobstore.Exists("abc")).ThenReturn(true, nil)

				router.ServeHTTP(responseWriter, httptest.NewRequest(
					"POST", "/app_stash/matches", strings.NewReader("[{\"sha1\":\"abc\"}, {\"sha1\":\"def\"}]")))

				Expect(*responseWriter).To(haveStatusCodeAndBody(
					Equal(http.StatusOK),
					Equal("[{\"sha1\":\"def\"}]")))
			})
		})
	})
})

func haveStatusCodeAndBody(statusCode types.GomegaMatcher, body types.GomegaMatcher) types.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"Code": statusCode,
		"Body": WithTransform(func(body *bytes.Buffer) string { return body.String() }, body),
	})
}

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
