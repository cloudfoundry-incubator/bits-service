package bitsgo_test

import (
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/petergtz/pegomock"

	"github.com/petergtz/bitsgo"

	. "github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/httputil"

	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"io"

	. "github.com/petergtz/pegomock"
)

var _ = Describe("ResourceHandler", func() {
	var (
		blobstore      *MockBlobstore
		handler        *bitsgo.ResourceHandler
		updater        *MockUpdater
		responseWriter *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		blobstore = NewMockBlobstore()
		updater = NewMockUpdater()
		handler = NewResourceHandlerWithUpdater(blobstore, updater, "test-resource", NewMockMetricsService(), 0)
		responseWriter = httptest.NewRecorder()
	})

	Context("Put", func() {
		It("translates NoSpaceLeftError into StatusInsufficientStorage", func() {
			When(blobstore.Put(AnyString(), anyReadSeeker())).ThenReturn(NewNoSpaceLeftError())

			handler.AddOrReplace(responseWriter,
				newTestRequest("test-resource", "some-filename", "some body"),
				map[string]string{})

			Expect(responseWriter.Code).To(Equal(http.StatusInsufficientStorage))
		})

		Context("async=true", func() {
			FIt("upload asynchronously", func() {

				synchronization := make(chan bool)

				When(blobstore.Put(AnyString(), anyReadSeeker())).Then(func(params []Param) ReturnValues {
					<-synchronization
					return nil
				})

				req := newTestRequest("test-resource", "some-filename", "some body")
				q, e := url.ParseQuery("async=true")
				Expect(e).NotTo(HaveOccurred())
				req.URL.RawQuery = q.Encode()
				handler.AddOrReplace(responseWriter,
					req,
					map[string]string{})

				updater.VerifyWasCalledOnce().NotifyProcessingUpload(AnyString())

				updater.VerifyWasCalled(Never()).NotifyUploadFailed(AnyString(), anyError())
				updater.VerifyWasCalled(Never()).NotifyUploadSucceeded(AnyString(), AnyString(), AnyString())
				synchronization <- true

				Eventually(func() []string {
					// TODO can this be done better?
					return interceptPegomockFailures(func() {
						updater.VerifyWasCalled(Never()).NotifyUploadFailed(AnyString(), anyError())
						updater.VerifyWasCalledOnce().NotifyUploadSucceeded(AnyString(), AnyString(), AnyString())
					})
				}, "2s").Should(BeEmpty())

				Expect(responseWriter.Code).To(Equal(http.StatusAccepted))
			})
		})
	})

	Context("Get", func() {
		Context("No If-None-Modify	 provided in request", func() {
			It("returns a response with body and StatusOK", func() {
				When(blobstore.GetOrRedirect(AnyString())).ThenReturn(ioutil.NopCloser(strings.NewReader("hello")), "", nil)

				handler.Get(responseWriter, newGetRequestWithOptionalIfNoneModify(""), nil)

				Expect(responseWriter.Code).To(Equal(http.StatusOK))
				Expect(responseWriter.Body.String()).To(Equal("hello"))
			})
		})

		Context("If-None-Modify provided in request", func() {
			BeforeEach(func() {
				When(blobstore.GetOrRedirect(AnyString())).ThenReturn(ioutil.NopCloser(strings.NewReader("hello")), "", nil)

				handler.Get(responseWriter, newGetRequestWithOptionalIfNoneModify(""), nil)

				Expect(responseWriter.Code).To(Equal(http.StatusOK))
			})

			Context("matches ETag", func() {
				It("returns a response with empty body and StatusNotModified", func() {
					When(blobstore.GetOrRedirect(AnyString())).ThenReturn(ioutil.NopCloser(strings.NewReader("hello")), "", nil)

					responseWriterFollowUpRequest := httptest.NewRecorder()

					handler.Get(
						responseWriterFollowUpRequest,
						newGetRequestWithOptionalIfNoneModify(responseWriter.HeaderMap.Get("ETag")),
						nil)

					Expect(responseWriterFollowUpRequest.Code).To(Equal(http.StatusNotModified))
					Expect(responseWriterFollowUpRequest.Body.String()).To(BeEmpty())
				})
			})
			Context("does not match ETag because content of blob has changed", func() {
				It("returns a response with body and StatusOK", func() {
					When(blobstore.GetOrRedirect(AnyString())).
						ThenReturn(ioutil.NopCloser(strings.NewReader("hello - the content has changed")), "", nil)

					r, e := http.NewRequest("GET", "irrelevant", nil)
					Expect(e).NotTo(HaveOccurred())
					r.Header.Set("If-None-Modify", responseWriter.HeaderMap.Get("ETag"))

					responseWriterFollowUpRequest := httptest.NewRecorder()

					handler.Get(
						responseWriterFollowUpRequest,
						newGetRequestWithOptionalIfNoneModify(responseWriter.HeaderMap.Get("ETag")),
						nil)

					Expect(responseWriter.Code).To(Equal(http.StatusOK))
					Expect(responseWriter.Body.String()).To(Equal("hello"))
				})
			})
		})
	})

	Context("Updater", func() {
		Context("No errors", func() {
			It("calls updater and blobstore in the right order", func() {
				handler.AddOrReplace(responseWriter,
					newTestRequest("test-resource", "some-filename", "some body"),
					map[string]string{"identifier": "someguid"})

				inOrderContext := new(InOrderContext)
				updater.VerifyWasCalledInOrder(Once(), inOrderContext).NotifyProcessingUpload("someguid")
				blobstore.VerifyWasCalledInOrder(Once(), inOrderContext).Put(EqString("someguid"), anyReadSeeker())
				updater.VerifyWasCalledInOrder(Once(), inOrderContext).NotifyUploadSucceeded(
					"someguid",
					// SHAs generated using shasum CLI:
					"754e8afdb33e180fbb7311eba784c5416766aa1c",
					"5f483264496cf1440c6ef569cc4fb9785d3bed896efdadfc998e9cb1badcec81")

				Expect(responseWriter.Code).To(Equal(http.StatusCreated))
			})
		})

		Context("Rejects an update", func() {
			Context("NotifyProcessingUpload returns NewStateForbiddenError", func() {
				It("does not upload the resource, returns BadRequest", func() {
					When(updater.NotifyProcessingUpload(AnyString())).ThenReturn(NewStateForbiddenError())

					handler.AddOrReplace(responseWriter,
						newTestRequest("test-resource", "some-filename", "some body"),
						map[string]string{"identifier": "someguid"})

					updater.VerifyWasCalled(Never()).NotifyUploadFailed(AnyString(), anyError())
					updater.VerifyWasCalled(Never()).NotifyUploadSucceeded(AnyString(), AnyString(), AnyString())
					blobstore.VerifyWasCalled(Never()).Put(AnyString(), anyReadSeeker())

					Expect(responseWriter.Code).To(Equal(http.StatusBadRequest))
					Expect(responseWriter.Body.String()).To(Equal(`{"description":"Cannot update an existing package.","code":290008}`))
				})
			})

			Context("NotifyUploadSucceeded returns an error", func() {
				It("has uploaded the resource, returns internal server error", func() {
					When(updater.NotifyUploadSucceeded(AnyString(), AnyString(), AnyString())).ThenReturn(fmt.Errorf("Some error"))

					handler.AddOrReplace(responseWriter,
						newTestRequest("test-resource", "some-filename", "some body"),
						map[string]string{"identifier": "someguid"})

					updater.VerifyWasCalled(Never()).NotifyUploadFailed(AnyString(), anyError())
					blobstore.VerifyWasCalledOnce().Put(EqString("someguid"), anyReadSeeker())
					Expect(responseWriter.Code).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("NotifyUploadFailed returns an error", func() {
				It("returns InternalServerError", func() {
					When(blobstore.Put(AnyString(), anyReadSeeker())).ThenReturn(fmt.Errorf("Some blobstore error"))
					When(updater.NotifyUploadFailed(AnyString(), anyError())).ThenReturn(fmt.Errorf("Some error"))

					handler.AddOrReplace(responseWriter,
						newTestRequest("test-resource", "some-filename", "some body"),
						map[string]string{"identifier": "someguid"})

					inOrderContext := new(InOrderContext)
					updater.VerifyWasCalledInOrder(Once(), inOrderContext).NotifyProcessingUpload("someguid")
					blobstore.VerifyWasCalledInOrder(Once(), inOrderContext).Put(EqString("someguid"), anyReadSeeker())
					updater.VerifyWasCalledInOrder(Once(), inOrderContext).NotifyUploadFailed(EqString("someguid"), anyError())
					Expect(responseWriter.Code).To(Equal(http.StatusInternalServerError))
				})
			})

		})

		Context("replies with an unexpected error", func() {
			Context("NotifyProcessingUpload returns unexpected error", func() {
				It("does not upload the resource, returns BadRequest", func() {
					When(updater.NotifyProcessingUpload(AnyString())).ThenReturn(fmt.Errorf("Unexpected error"))

					handler.AddOrReplace(responseWriter,
						newTestRequest("test-resource", "some-filename", "some body"),
						map[string]string{"identifier": "someguid"})

					updater.VerifyWasCalled(Never()).NotifyUploadFailed(AnyString(), anyError())
					updater.VerifyWasCalled(Never()).NotifyUploadSucceeded(AnyString(), AnyString(), AnyString())
					blobstore.VerifyWasCalled(Never()).Put(AnyString(), anyReadSeeker())

					Expect(responseWriter.Code).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("replies with guid not found", func() {
			Context("NotifyProcessingUpload returns guid not found", func() {
				It("does not upload the resource, returns ResourceNotFound", func() {
					When(updater.NotifyProcessingUpload(AnyString())).ThenReturn(bitsgo.NewNotFoundError())

					handler.AddOrReplace(responseWriter,
						newTestRequest("test-resource", "some-filename", "some body"),
						map[string]string{"identifier": "someguid"})

					updater.VerifyWasCalled(Never()).NotifyUploadFailed(AnyString(), anyError())
					updater.VerifyWasCalled(Never()).NotifyUploadSucceeded(AnyString(), AnyString(), AnyString())
					blobstore.VerifyWasCalled(Never()).Put(AnyString(), anyReadSeeker())

					Expect(responseWriter.Code).To(Equal(http.StatusNotFound))
				})
			})
		})
	})
})

func anyReadSeeker() io.ReadSeeker {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*io.ReadSeeker)(nil)).Elem()))
	return nil
}

func anyError() error {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*error)(nil)).Elem()))
	return nil
}

func newTestRequest(resource string, filename string, body string) *http.Request {
	request, e := httputil.NewPutRequest("http://notrelevant",
		map[string]map[string]io.Reader{
			"test-resource": map[string]io.Reader{"some-filename": strings.NewReader("some body")},
		})
	Expect(e).NotTo(HaveOccurred())
	return request
}

func newGetRequestWithOptionalIfNoneModify(ifNoneModify string) *http.Request {
	r, e := http.NewRequest("GET", "irrelevant", nil)
	Expect(e).NotTo(HaveOccurred())
	if ifNoneModify != "" {
		r.Header.Set("If-None-Modify", ifNoneModify)
	}
	return r
}

func interceptPegomockFailures(f func()) []string {
	originalHandler := pegomock.GlobalFailHandler
	failures := []string{}
	RegisterMockFailHandler(func(message string, callerSkip ...int) {
		failures = append(failures, message)
	})
	f()
	pegomock.RegisterMockFailHandler(originalHandler)
	return failures
}
