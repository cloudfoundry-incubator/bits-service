package bitsgo_test

import (
	"archive/zip"
	"errors"
	"io/ioutil"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo"
	. "github.com/petergtz/bitsgo/matchers"
	. "github.com/petergtz/pegomock"
)

var _ = Describe("AppStash", func() {
	var (
		blobstore       *MockNoRedirectBlobstore
		appStashHandler *bitsgo.AppStashHandler
	)

	BeforeEach(func() {
		blobstore = NewMockNoRedirectBlobstore()
		appStashHandler = bitsgo.NewAppStashHandler(blobstore, 0)
	})

	Describe("CreateTempZipFileFrom", func() {
		It("Creates a zip", func() {
			When(blobstore.Get("abc")).ThenReturn(ioutil.NopCloser(strings.NewReader("filename1 content")), nil)

			tempFileName, e := appStashHandler.CreateTempZipFileFrom([]bitsgo.BundlesPayload{
				bitsgo.BundlesPayload{
					Sha1: "abc",
					Fn:   "filename1",
					Mode: "644",
				},
			})
			Expect(e).NotTo(HaveOccurred())

			reader, e := zip.OpenReader(tempFileName)
			Expect(e).NotTo(HaveOccurred())
			Expect(reader.File).To(HaveLen(1))
			verifyZipFileEntry(reader, 0, "filename1", "filename1 content")
		})

		Context("One error from blobstore", func() {
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
					})
					Expect(e).NotTo(HaveOccurred())

					reader, e := zip.OpenReader(tempFileName)
					Expect(e).NotTo(HaveOccurred())
					Expect(reader.File).To(HaveLen(1))
					verifyZipFileEntry(reader, 0, "filename1", "filename1 content")
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
					})
					Expect(e).NotTo(HaveOccurred())

					reader, e := zip.OpenReader(tempFileName)
					Expect(e).NotTo(HaveOccurred())
					Expect(reader.File).To(HaveLen(2))
					verifyZipFileEntry(reader, 0, "filename1", "filename1 content")
					verifyZipFileEntry(reader, 1, "filename2", "filename2 content")
				})
			})
		})
	})
})

func verifyZipFileEntry(reader *zip.ReadCloser, index int, expectedFilename string, expectedContent string) {
	Expect(reader.File[index].Name).To(Equal(expectedFilename))
	content, e := reader.File[index].Open()
	Expect(e).NotTo(HaveOccurred())
	Expect(ioutil.ReadAll(content)).To(MatchRegexp(expectedContent))
}
