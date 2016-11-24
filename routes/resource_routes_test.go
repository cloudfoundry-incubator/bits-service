package routes_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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

//go:generate pegomock generate --use-experimental-model-gen --package main_test Blobstore

func TestRoutes(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	pegomock.RegisterMockFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Routes")
}

var _ = Describe("routes", func() {
	Describe("/packages/{guid}", func() {
		ItSupportsMethodsGetPutDeleteFor("packages", "package", SetUpPackageRoutes)
	})

	Describe("/droplets/{guid}", func() {
		ItSupportsMethodsGetPutDeleteFor("droplets", "droplet", SetUpDropletRoutes)
	})

	Describe("/buildpacks/{guid}", func() {
		ItSupportsMethodsGetPutDeleteFor("buildpacks", "buildpack", SetUpBuildpackRoutes)
	})
})

func ItSupportsMethodsGetPutDeleteFor(routeName string, resourceType string, setUp func(router *mux.Router, blobstore Blobstore)) {
	var (
		blobstore      *MockBlobstore
		router         *mux.Router
		responseWriter *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		blobstore = NewMockBlobstore()
		router = mux.NewRouter()
		responseWriter = httptest.NewRecorder()
		setUp(router, blobstore)
	})

	Context("Method GET", func() {
		It("returns StatusNotFound when blobstore returns NotFoundError", func() {
			When(func() { blobstore.Get(AnyString()) }).
				ThenReturn(nil, "", NewNotFoundError())

			router.ServeHTTP(responseWriter, httptest.NewRequest("GET", "/"+routeName+"/theguid", nil))

			Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
		})

		It("returns StatusOK and fills body with contents from file located at the paritioned path", func() {
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

			// _, fileContent, _ := blobstore.VerifyWasCalledOnce().Put(EqString("/th/eg/theguid"), AnyReadSeeker(), AnyResponseWriter()).GetCapturedArguments()
			// Expect(ioutil.ReadAll(fileContent)).To(MatchRegexp("My test string"))
		})
	})
}

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
	bodyBuf := &bytes.Buffer{}
	header, e := httputil.AddFormFileTo(bodyBuf, formFiles)
	Expect(e).NotTo(HaveOccurred())
	request := httptest.NewRequest("PUT", path, bodyBuf)
	httputil.AddHeaderTo(request, header)
	return request
}

func AnyResponseWriter() http.ResponseWriter {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()))
	return nil
}

func AnyReadSeeker() io.ReadSeeker {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*io.ReadSeeker)(nil)).Elem()))
	return nil
}
