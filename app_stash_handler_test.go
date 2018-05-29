package bitsgo_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
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
	. "github.com/petergtz/bitsgo/matchers"
	. "github.com/petergtz/pegomock"
)

var _ = Describe("AppStash", func() {
	var (
		blobstore       *inmemory.Blobstore
		appStashHandler *bitsgo.AppStashHandler
		responseWriter  *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		blobstore = inmemory.NewBlobstore()
		appStashHandler = bitsgo.NewAppStashHandler(blobstore, 0)
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

				zipFile, e := os.Open(filepath.Join(filepath.Dir(filename), "asset", "test-file.zip"))
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
				verifyZipFileEntry(zipReader, "filenameA", "cached content")
				verifyZipFileEntry(zipReader, "filenameB", "test-content")
				verifyZipFileEntry(zipReader, "folder/filenameC", "another cached content")
				verifyZipFileEntry(zipReader, "zip-folder/file-in-folder", "folder file content")

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

					zipFile, e := os.Open(filepath.Join(filepath.Dir(filename), "asset", "test-file.zip"))
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
					verifyZipFileEntry(zipReader, "filenameB", "test-content")
					verifyZipFileEntry(zipReader, "zip-folder/file-in-folder", "folder file content")

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
					verifyZipFileEntry(zipReader, "anotherFilenameB", "test-content")
					verifyZipFileEntry(zipReader, "zip-folder/another-file-in-folder", "folder file content")
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

	Describe("CreateTempZipFileFrom", func() {
		It("Creates a zip", func() {
			Expect(blobstore.Put("abc", strings.NewReader("filename1 content"))).To(Succeed())

			tempFileName, e := appStashHandler.CreateTempZipFileFrom([]bitsgo.Fingerprint{
				bitsgo.Fingerprint{
					Sha1: "abc",
					Fn:   "filename1",
					Mode: "644",
				},
			}, nil)
			Expect(e).NotTo(HaveOccurred())

			reader, e := zip.OpenReader(tempFileName)
			Expect(e).NotTo(HaveOccurred())
			Expect(reader.File).To(HaveLen(1))
			verifyZipFileEntry(&reader.Reader, "filename1", "filename1 content")
		})

		Context("One error from blobstore", func() {
			var blobstore *MockNoRedirectBlobstore

			BeforeEach(func() {
				blobstore = NewMockNoRedirectBlobstore()
				appStashHandler = bitsgo.NewAppStashHandler(blobstore, 0)
			})

			Context("Error in Blobstore.Get", func() {
				It("Retries and creates the zip successfully", func() {
					When(blobstore.Get("abc")).
						ThenReturn(nil, errors.New("Some error")).
						ThenReturn(ioutil.NopCloser(strings.NewReader("filename1 content")), nil)

					tempFileName, e := appStashHandler.CreateTempZipFileFrom([]bitsgo.Fingerprint{
						bitsgo.Fingerprint{
							Sha1: "abc",
							Fn:   "filename1",
							Mode: "644",
						},
					}, nil)
					Expect(e).NotTo(HaveOccurred())

					reader, e := zip.OpenReader(tempFileName)
					Expect(e).NotTo(HaveOccurred())
					Expect(reader.File).To(HaveLen(1))
					verifyZipFileEntry(&reader.Reader, "filename1", "filename1 content")
				})
			})

			Context("Error in read", func() {
				It("Retries and creates the zip successfully", func() {
					readClose := NewMockReadCloser()
					When(readClose.Read(AnySliceOfByte())).ThenReturn(1, errors.New("some random read error"))

					When(blobstore.Get("abc")).
						ThenReturn(readClose, nil).
						ThenReturn(ioutil.NopCloser(strings.NewReader("filename1 content")), nil)

					When(blobstore.Get("def")).
						ThenReturn(readClose, nil).
						ThenReturn(ioutil.NopCloser(strings.NewReader("filename2 content")), nil)

					tempFileName, e := appStashHandler.CreateTempZipFileFrom([]bitsgo.Fingerprint{
						bitsgo.Fingerprint{
							Sha1: "abc",
							Fn:   "filename1",
							Mode: "644",
						},
						bitsgo.Fingerprint{
							Sha1: "def",
							Fn:   "filename2",
							Mode: "644",
						},
					}, nil)
					Expect(e).NotTo(HaveOccurred())

					reader, e := zip.OpenReader(tempFileName)
					Expect(e).NotTo(HaveOccurred())
					Expect(reader.File).To(HaveLen(2))
					verifyZipFileEntry(&reader.Reader, "filename1", "filename1 content")
					verifyZipFileEntry(&reader.Reader, "filename2", "filename2 content")
				})
			})
		})

		Context("maximumSize and minimumSize provided", func() {
			BeforeEach(func() {
				appStashHandler = bitsgo.NewAppStashHandlerWithSizeThresholds(blobstore, 0, 15, 30)
			})

			It("only stores the file which is within range of thresholds", func() {
				_, filename, _, _ := runtime.Caller(0)

				zipFile, e := os.Open(filepath.Join(filepath.Dir(filename), "asset", "test-file.zip"))
				Expect(e).NotTo(HaveOccurred())
				defer zipFile.Close()

				openZipFile, e := zip.OpenReader(zipFile.Name())
				Expect(e).NotTo(HaveOccurred())
				defer openZipFile.Close()

				tempFilename, e := appStashHandler.CreateTempZipFileFrom([]bitsgo.Fingerprint{}, &openZipFile.Reader)
				Expect(e).NotTo(HaveOccurred())
				os.Remove(tempFilename)

				Expect(blobstore.Entries).To(HaveLen(1))
				Expect(blobstore.Entries).To(HaveKeyWithValue("e04c62ab0e87c29f862ee7c4e85c9fed51531dae", []byte("folder file content\n")))
			})
		})
	})
})

func verifyZipFileEntry(reader *zip.Reader, expectedFilename string, expectedContent string) {
	var foundEntries []string
	for _, entry := range reader.File {
		if entry.Name == expectedFilename {
			content, e := entry.Open()
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(content)).To(MatchRegexp(expectedContent), "for filename "+expectedFilename)
			return
		}
		foundEntries = append(foundEntries, entry.Name)
	}
	Fail("Did not find entry with name " + expectedFilename + ". Found only: " + strings.Join(foundEntries, ", "))
}
