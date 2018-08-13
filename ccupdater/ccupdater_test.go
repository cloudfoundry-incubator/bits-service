package ccupdater_test

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry-incubator/bits-service"

	. "github.com/cloudfoundry-incubator/bits-service/ccupdater"
	. "github.com/cloudfoundry-incubator/bits-service/ccupdater/matchers"
	. "github.com/petergtz/pegomock"
)

var _ = Describe("CCUpdater", func() {
	var (
		httpClient *MockHttpClient
		updater    *CCUpdater
	)

	BeforeEach(func() {
		httpClient = NewMockHttpClient()
		updater = NewCCUpdaterWithHttpClient("http://example.com/some/endpoint", "PATCH", httpClient)
	})

	Describe("NotifyProcessingUpload", func() {
		It("works", func() {
			When(httpClient.Do(AnyPtrToHttpRequest())).ThenReturn(&http.Response{}, nil)

			e := updater.NotifyProcessingUpload("abc")

			Expect(e).NotTo(HaveOccurred())

			request := httpClient.VerifyWasCalledOnce().Do(AnyPtrToHttpRequest()).GetCapturedArguments()
			Expect(request.Method).To(Equal("PATCH"))
			Expect(request.URL.String()).To(Equal("http://example.com/some/endpoint/abc"))
			Expect(ioutil.ReadAll(request.Body)).To(MatchJSON(`{"state":"PROCESSING_UPLOAD"}`))
		})

		Context("http client returns some generic error", func() {
			It("fails with a generic error", func() {
				When(httpClient.Do(AnyPtrToHttpRequest())).ThenReturn(nil, fmt.Errorf("Some network error"))

				e := updater.NotifyProcessingUpload("abc")

				Expect(e).To(MatchError(SatisfyAll(
					ContainSubstring("Could not make request against CC"),
					ContainSubstring("abc"),
					ContainSubstring("Some network error"),
				)))
			})
		})

		Context("http client returns NotFound", func() {
			It("fails with a generic error", func() {
				When(httpClient.Do(AnyPtrToHttpRequest())).ThenReturn(&http.Response{StatusCode: http.StatusNotFound}, nil)

				e := updater.NotifyProcessingUpload("abc")

				Expect(e).To(Equal(bitsgo.NewNotFoundError()))
			})
		})
	})

	Describe("NotifyUploadSucceeded", func() {
		It("works", func() {
			When(httpClient.Do(AnyPtrToHttpRequest())).ThenReturn(&http.Response{}, nil)

			e := updater.NotifyUploadSucceeded("abc", "sha1", "sha256")

			Expect(e).NotTo(HaveOccurred())

			request := httpClient.VerifyWasCalledOnce().Do(AnyPtrToHttpRequest()).GetCapturedArguments()
			Expect(request.Method).To(Equal("PATCH"))
			Expect(request.URL.String()).To(Equal("http://example.com/some/endpoint/abc"))
			Expect(ioutil.ReadAll(request.Body)).To(MatchJSON(`{
				"state": "READY",
				"checksums": [
				  {
					"type": "sha1",
					"value": "sha1"
				  },
				  {
					"type": "sha256",
					"value": "sha256"
				  }
				]
			  }`))
		})
	})

	Describe("NotifyUploadFailed", func() {
		It("works", func() {
			When(httpClient.Do(AnyPtrToHttpRequest())).ThenReturn(&http.Response{}, nil)

			e := updater.NotifyUploadFailed("abc", fmt.Errorf("some error"))

			Expect(e).NotTo(HaveOccurred())

			request := httpClient.VerifyWasCalledOnce().Do(AnyPtrToHttpRequest()).GetCapturedArguments()
			Expect(request.Method).To(Equal("PATCH"))
			Expect(request.URL.String()).To(Equal("http://example.com/some/endpoint/abc"))
			Expect(ioutil.ReadAll(request.Body)).To(MatchJSON(`{
				"state": "FAILED",
				"error": "some error"
			  }`))
		})
	})
})
