package bitsgo_test

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo"
	inmemory "github.com/petergtz/bitsgo/blobstores/inmemory"
	"github.com/petergtz/bitsgo/httputil"
	. "github.com/petergtz/bitsgo/testutil"
)

var _ = Describe("AppStash", func() {
	var (
		blobstore       *inmemory.Blobstore
		appStashHandler *bitsgo.AppStashHandler
		responseWriter  *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		blobstore = inmemory.NewBlobstore()
		appStashHandler = bitsgo.NewAppStashHandlerWithSizeThresholds(blobstore, 0, 0, math.MaxUint64)
		responseWriter = httptest.NewRecorder()
	})

	Describe("PostMatches", func() {
		Context("maximumSize and minimumSize provided", func() {

			const (
				minimumSize          = 15
				maximumSize          = 30
				sizeWithinThresholds = "20"
				sizeAboveThreshold   = "999"
				sizeBelowThreshold   = "1"
			)

			BeforeEach(func() {
				appStashHandler = bitsgo.NewAppStashHandlerWithSizeThresholds(blobstore, 0, minimumSize, maximumSize)
				Expect(blobstore.Put("shaA", strings.NewReader("cached content"))).To(Succeed())
				Expect(blobstore.Put("shaB", strings.NewReader("another cached content"))).To(Succeed())
				Expect(blobstore.Put("shaC", strings.NewReader("yet another cached content"))).To(Succeed())
			})

			It("matches only files where sizes are within thresholds", func() {
				appStashHandler.PostMatches(responseWriter, httptest.NewRequest(
					"POST", "http://example.com",
					strings.NewReader(`[
						{
							"sha1":"shaA",
							"fn":"filenameA",
							"size": `+sizeWithinThresholds+`,
							"mode": "644"
						},
						{
							"sha1":"shaB",
							"fn":"filenameB",
							"size": `+sizeAboveThreshold+`,
							"mode": "644"
						},
						{
							"sha1":"shaC",
							"fn":"filenameC",
							"size": `+sizeBelowThreshold+`,
							"mode": "644"
						}
					]`)))

				Expect(responseWriter.Code).To(Equal(http.StatusOK), responseWriter.Body.String())
				Expect(responseWriter.Body.String()).To(MatchJSON(`[
					{
						"sha1":"shaA",
						"fn":"filenameA",
						"size": ` + sizeWithinThresholds + `,
						"mode": "644"
					}
					]`))
			})
		})
	})

	Describe("PostBundles", func() {

		BeforeEach(func() {
			Expect(blobstore.Put("shaA", strings.NewReader("cached content"))).To(Succeed())
			Expect(blobstore.Put("shaC", strings.NewReader("another cached content"))).To(Succeed())
		})

		Context("non-multipart/form-data request", func() {
			It("bundles all files from blobstore into zip bundle", func() {

				// TODO this should be a POST request, but it doesn't really matter
				r := httptest.NewRequest("POST", "http://example.com", strings.NewReader(`[
						{
							"sha1":"shaA",
							"fn":"filenameA"
						}
					]`))

				appStashHandler.PostBundles(responseWriter, r)

				Expect(responseWriter.Code).To(Equal(http.StatusOK), responseWriter.Body.String())
				zipReader, e := zip.NewReader(bytes.NewReader(responseWriter.Body.Bytes()), int64(responseWriter.Body.Len()))
				Expect(e).NotTo(HaveOccurred())

				Expect(zipReader.File[0].Name).To(Equal("filenameA"))
				zipContent, e := zipReader.File[0].Open()
				Expect(e).NotTo(HaveOccurred())
				Expect(ioutil.ReadAll(zipContent)).To(MatchRegexp("cached content"))
			})
		})

		Context("multipart/form-data request", func() {
			It("bundles all files from blobstore and from uploaded zip into zip bundle", func() {
				_, filename, _, _ := runtime.Caller(0)

				zipFile, e := os.Open(filepath.Join(filepath.Dir(filename), "assets", "test-file.zip"))
				Expect(e).NotTo(HaveOccurred())
				defer zipFile.Close()

				// TODO this should be a POST request, but it doesn't really matter
				r, e := httputil.NewPutRequest("some url", map[string]map[string]io.Reader{
					"resources": map[string]io.Reader{"irrelevant": strings.NewReader(`[
						{
							"sha1":"shaA",
							"fn":"filenameA"
						},
						{
							"sha1":"shaC",
							"fn":"folder/filenameC"
						}
						]`)},
					"application": map[string]io.Reader{"irrelevant": zipFile},
				})
				Expect(e).NotTo(HaveOccurred())

				appStashHandler.PostBundles(responseWriter, r)

				Expect(responseWriter.Code).To(Equal(http.StatusOK), responseWriter.Body.String())
				zipReader, e := zip.NewReader(bytes.NewReader(responseWriter.Body.Bytes()), int64(responseWriter.Body.Len()))
				Expect(e).NotTo(HaveOccurred())

				Expect(zipReader.File).To(HaveLen(4))
				VerifyZipFileEntry(zipReader, "filenameA", "cached content")
				VerifyZipFileEntry(zipReader, "filenameB", "test-content")
				VerifyZipFileEntry(zipReader, "folder/filenameC", "another cached content")
				VerifyZipFileEntry(zipReader, "zip-folder/file-in-folder", "folder file content")

				content, e := blobstore.Get("b971c6ef19b1d70ae8f0feb989b106c319b36230")
				Expect(e).NotTo(HaveOccurred())
				Expect(ioutil.ReadAll(content)).To(MatchRegexp("test-content\n"))
				content, e = blobstore.Get("e04c62ab0e87c29f862ee7c4e85c9fed51531dae")
				Expect(e).NotTo(HaveOccurred())
				Expect(ioutil.ReadAll(content)).To(MatchRegexp("folder file content\n"))
			})

			Describe("App-stash caching round-trip", func() {
				It("can re-use cached files from previous bundles", func() {
					_, filename, _, _ := runtime.Caller(0)

					zipFile, e := os.Open(filepath.Join(filepath.Dir(filename), "assets", "test-file.zip"))
					Expect(e).NotTo(HaveOccurred())
					defer zipFile.Close()

					r, e := httputil.NewPutRequest("some url", map[string]map[string]io.Reader{
						"resources":   map[string]io.Reader{"irrelevant": strings.NewReader(`[]`)},
						"application": map[string]io.Reader{"irrelevant": zipFile},
					})
					Expect(e).NotTo(HaveOccurred())

					appStashHandler.PostBundles(responseWriter, r)

					Expect(responseWriter.Code).To(Equal(http.StatusOK), responseWriter.Body.String())
					zipReader, e := zip.NewReader(bytes.NewReader(responseWriter.Body.Bytes()), int64(responseWriter.Body.Len()))
					Expect(e).NotTo(HaveOccurred())

					Expect(zipReader.File).To(HaveLen(2))
					VerifyZipFileEntry(zipReader, "filenameB", "test-content")
					VerifyZipFileEntry(zipReader, "zip-folder/file-in-folder", "folder file content")

					responseWriter = httptest.NewRecorder()
					appStashHandler.PostBundles(responseWriter, httptest.NewRequest("POST", "http://example.com", strings.NewReader(`[
						{
							"sha1":"b971c6ef19b1d70ae8f0feb989b106c319b36230",
							"fn":"anotherFilenameB"
						},
						{
							"sha1":"e04c62ab0e87c29f862ee7c4e85c9fed51531dae",
							"fn":"zip-folder/another-file-in-folder"
						}
					]`)))

					Expect(responseWriter.Code).To(Equal(http.StatusOK), responseWriter.Body.String())
					zipReader, e = zip.NewReader(bytes.NewReader(responseWriter.Body.Bytes()), int64(responseWriter.Body.Len()))
					Expect(e).NotTo(HaveOccurred())

					Expect(zipReader.File).To(HaveLen(2))
					VerifyZipFileEntry(zipReader, "anotherFilenameB", "test-content")
					VerifyZipFileEntry(zipReader, "zip-folder/another-file-in-folder", "folder file content")
				})
			})

			Context("application form parameter is missing", func() {
				It("returns StatusBadRequest", func() {
					r, e := httputil.NewPutRequest("some url", map[string]map[string]io.Reader{
						"resources": map[string]io.Reader{"irrelevant": strings.NewReader(`[
							{
								"sha1":"shaA",
								"fn":"filenameA"
							},
							{
								"sha1":"shaB",
								"fn":"filenameB"
							}
						]`)}})
					Expect(e).NotTo(HaveOccurred())

					appStashHandler.PostBundles(responseWriter, r)

					Expect(responseWriter.Code).To(Equal(http.StatusBadRequest), responseWriter.Body.String())
				})
			})
		})

	})
})
