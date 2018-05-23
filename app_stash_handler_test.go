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

			tempFileName, e := appStashHandler.CreateTempZipFileFrom([]bitsgo.BundlesPayload{
				bitsgo.BundlesPayload{
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

					tempFileName, e := appStashHandler.CreateTempZipFileFrom([]bitsgo.BundlesPayload{
						bitsgo.BundlesPayload{
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

					tempFileName, e := appStashHandler.CreateTempZipFileFrom([]bitsgo.BundlesPayload{
						bitsgo.BundlesPayload{
							Sha1: "abc",
							Fn:   "filename1",
							Mode: "644",
						},
						bitsgo.BundlesPayload{
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
	})
})

func verifyZipFileEntry(reader *zip.Reader, expectedFilename string, expectedContent string) {
	for _, entry := range reader.File {
		if entry.Name == expectedFilename {
			content, e := entry.Open()
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(content)).To(MatchRegexp(expectedContent), "for filename "+expectedFilename)
			return
		}
	}
	Fail("Did not find entry with name " + expectedFilename)
}
