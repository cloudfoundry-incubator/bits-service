package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"

	"github.com/cloudfoundry-incubator/bits-service/middlewares"
	. "github.com/cloudfoundry-incubator/bits-service/testutil"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/petergtz/pegomock"
	"github.com/urfave/negroni"
)

var _ = Describe("BasicAuthMiddle", func() {

	var (
		server     *httptest.Server
		middleware *middlewares.BasicAuthMiddleware
		mux        *http.ServeMux
	)

	BeforeEach(func() {
		mux = http.NewServeMux()
		mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			rw.Write([]byte("Hello"))
		})
		middleware = middlewares.NewBasicAuthMiddleWare(
			middlewares.Credential{
				Username: "the-username",
				Password: "the-password",
			},
			middlewares.Credential{
				Username: "another-username",
				Password: "another-password",
			},
		)
	})

	JustBeforeEach(func() {
		server = httptest.NewServer(negroni.New(
			middleware,
			negroni.Wrap(mux),
		))
	})

	AfterEach(func() {
		server.Close()
	})

	It("returns status OK and the body from the handler when basic auth is valid", func() {
		request := newGetRequest(server.URL)
		request.SetBasicAuth("the-username", "the-password")

		response, e := http.DefaultClient.Do(request)
		Expect(e).NotTo(HaveOccurred())

		Expect(*response).To(HaveStatusCodeAndBody(
			Equal(http.StatusOK),
			MatchRegexp("Hello")))

		request = newGetRequest(server.URL)
		request.SetBasicAuth("another-username", "another-password")

		response, e = http.DefaultClient.Do(request)
		Expect(e).NotTo(HaveOccurred())

		Expect(*response).To(HaveStatusCodeAndBody(
			Equal(http.StatusOK),
			MatchRegexp("Hello")))
		Expect(response.Header.Get("WWW-Authenticate")).To(BeEmpty())
	})

	It("returns status unauthorized when basic auth is invalid", func() {
		request := newGetRequest(server.URL)
		request.SetBasicAuth("the-username", "wrong-password")

		response, e := http.DefaultClient.Do(request)
		Expect(e).NotTo(HaveOccurred())

		Expect(*response).To(HaveStatusCodeAndBody(
			Equal(http.StatusUnauthorized),
			BeEmpty()))
		Expect(response.Header.Get("WWW-Authenticate")).To(ContainSubstring("Basic realm"))

	})

	Context("unauthorizedHandler is set", func() {
		It("uses the unauthorizedHandler", func() {
			mockHandler := NewMockHandler()
			middleware.WithUnauthorizedHandler(mockHandler)

			request := newGetRequest(server.URL)
			request.SetBasicAuth("the-username", "wrong-password")

			_, e := http.DefaultClient.Do(request)
			Expect(e).NotTo(HaveOccurred())

			mockHandler.VerifyWasCalledOnce().ServeHTTP(anyResponseWriter(), anyRequestPtr())
		})
	})

	It("returns status unauthorized when basic auth is not set", func() {
		request := newGetRequest(server.URL)

		response, e := http.DefaultClient.Do(request)
		Expect(e).NotTo(HaveOccurred())

		Expect(*response).To(HaveStatusCodeAndBody(
			Equal(http.StatusUnauthorized),
			BeEmpty()))
		Expect(response.Header.Get("WWW-Authenticate")).To(ContainSubstring("Basic realm"))
	})
})

func newGetRequest(url string) *http.Request {
	request, e := http.NewRequest("GET", url, nil)
	Expect(e).NotTo(HaveOccurred())
	return request
}

func anyResponseWriter() http.ResponseWriter {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()))
	return nil
}

func anyRequestPtr() *http.Request {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*http.Request)(nil))))
	return nil
}

// Declarations for Ginkgo DSL
type Done ginkgo.Done
type Benchmarker ginkgo.Benchmarker

var GinkgoWriter = ginkgo.GinkgoWriter
var GinkgoRandomSeed = ginkgo.GinkgoRandomSeed
var GinkgoParallelNode = ginkgo.GinkgoParallelNode
var GinkgoT = ginkgo.GinkgoT
var CurrentGinkgoTestDescription = ginkgo.CurrentGinkgoTestDescription
var RunSpecs = ginkgo.RunSpecs
var RunSpecsWithDefaultAndCustomReporters = ginkgo.RunSpecsWithDefaultAndCustomReporters
var RunSpecsWithCustomReporters = ginkgo.RunSpecsWithCustomReporters
var Skip = ginkgo.Skip
var Fail = ginkgo.Fail
var GinkgoRecover = ginkgo.GinkgoRecover
var Describe = ginkgo.Describe
var FDescribe = ginkgo.FDescribe
var PDescribe = ginkgo.PDescribe
var XDescribe = ginkgo.XDescribe
var Context = ginkgo.Context
var FContext = ginkgo.FContext
var PContext = ginkgo.PContext
var XContext = ginkgo.XContext
var It = ginkgo.It
var FIt = ginkgo.FIt
var PIt = ginkgo.PIt
var XIt = ginkgo.XIt
var Specify = ginkgo.Specify
var FSpecify = ginkgo.FSpecify
var PSpecify = ginkgo.PSpecify
var XSpecify = ginkgo.XSpecify
var By = ginkgo.By
var Measure = ginkgo.Measure
var FMeasure = ginkgo.FMeasure
var PMeasure = ginkgo.PMeasure
var XMeasure = ginkgo.XMeasure
var BeforeSuite = ginkgo.BeforeSuite
var AfterSuite = ginkgo.AfterSuite
var SynchronizedBeforeSuite = ginkgo.SynchronizedBeforeSuite
var SynchronizedAfterSuite = ginkgo.SynchronizedAfterSuite
var BeforeEach = ginkgo.BeforeEach
var JustBeforeEach = ginkgo.JustBeforeEach
var AfterEach = ginkgo.AfterEach

// Declarations for Gomega DSL
var RegisterFailHandler = gomega.RegisterFailHandler
var RegisterTestingT = gomega.RegisterTestingT
var InterceptGomegaFailures = gomega.InterceptGomegaFailures
var Ω = gomega.Ω
var Expect = gomega.Expect
var ExpectWithOffset = gomega.ExpectWithOffset
var Eventually = gomega.Eventually
var EventuallyWithOffset = gomega.EventuallyWithOffset
var Consistently = gomega.Consistently
var ConsistentlyWithOffset = gomega.ConsistentlyWithOffset
var SetDefaultEventuallyTimeout = gomega.SetDefaultEventuallyTimeout
var SetDefaultEventuallyPollingInterval = gomega.SetDefaultEventuallyPollingInterval
var SetDefaultConsistentlyDuration = gomega.SetDefaultConsistentlyDuration
var SetDefaultConsistentlyPollingInterval = gomega.SetDefaultConsistentlyPollingInterval
var NewGomegaWithT = gomega.NewGomegaWithT

// Declarations for Gomega Matchers
var Equal = gomega.Equal
var BeEquivalentTo = gomega.BeEquivalentTo
var BeIdenticalTo = gomega.BeIdenticalTo
var BeNil = gomega.BeNil
var BeTrue = gomega.BeTrue
var BeFalse = gomega.BeFalse
var HaveOccurred = gomega.HaveOccurred
var Succeed = gomega.Succeed
var MatchError = gomega.MatchError
var BeClosed = gomega.BeClosed
var Receive = gomega.Receive
var BeSent = gomega.BeSent
var MatchRegexp = gomega.MatchRegexp
var ContainSubstring = gomega.ContainSubstring
var HavePrefix = gomega.HavePrefix
var HaveSuffix = gomega.HaveSuffix
var MatchJSON = gomega.MatchJSON
var MatchXML = gomega.MatchXML
var MatchYAML = gomega.MatchYAML
var BeEmpty = gomega.BeEmpty
var HaveLen = gomega.HaveLen
var HaveCap = gomega.HaveCap
var BeZero = gomega.BeZero
var ContainElement = gomega.ContainElement
var ConsistOf = gomega.ConsistOf
var HaveKey = gomega.HaveKey
var HaveKeyWithValue = gomega.HaveKeyWithValue
var BeNumerically = gomega.BeNumerically
var BeTemporally = gomega.BeTemporally
var BeAssignableToTypeOf = gomega.BeAssignableToTypeOf
var Panic = gomega.Panic
var BeAnExistingFile = gomega.BeAnExistingFile
var BeARegularFile = gomega.BeARegularFile
var BeADirectory = gomega.BeADirectory
var And = gomega.And
var SatisfyAll = gomega.SatisfyAll
var Or = gomega.Or
var SatisfyAny = gomega.SatisfyAny
var Not = gomega.Not
var WithTransform = gomega.WithTransform
