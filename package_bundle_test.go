package bitsgo_test

import (
	"archive/zip"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	bitsgo "github.com/cloudfoundry-incubator/bits-service"
	inmemory "github.com/cloudfoundry-incubator/bits-service/blobstores/inmemory"
	"github.com/cloudfoundry-incubator/bits-service/logger"
	. "github.com/cloudfoundry-incubator/bits-service/matchers"
	. "github.com/cloudfoundry-incubator/bits-service/testutil"
	. "github.com/petergtz/pegomock"
	"github.com/pkg/errors"
)

var _ = Describe("CreateTempZipFileFrom", func() {
	var blobstore *inmemory.Blobstore

	BeforeEach(func() { blobstore = inmemory.NewBlobstore() })

	It("Creates a zip", func() {
		Expect(blobstore.Put("abc", strings.NewReader("filename1 content"))).To(Succeed())

		tempFileName, e := bitsgo.CreateTempZipFileFrom([]bitsgo.Fingerprint{
			bitsgo.Fingerprint{
				Sha1: "abc",
				Fn:   "filename1",
				Mode: "644",
			},
		}, nil, 0, math.MaxUint64, blobstore, NewMockMetricsService(), logger.Log)
		Expect(e).NotTo(HaveOccurred())

		reader, e := zip.OpenReader(tempFileName)
		Expect(e).NotTo(HaveOccurred())
		Expect(reader.File).To(HaveLen(1))
		VerifyZipFileEntry(&reader.Reader, "filename1", "filename1 content")
	})
	Context("Handles the 'Modified time' property", func() {

		var lastModifedFromTempFile time.Time

		BeforeEach(func() {
			Expect(blobstore.Put("abc", strings.NewReader("filename1 content"))).To(Succeed())

			var e error
			tempFileName, e := bitsgo.CreateTempZipFileFrom([]bitsgo.Fingerprint{
				bitsgo.Fingerprint{
					Sha1: "abc",
					Fn:   "filename1",
					Mode: "644",
				},
			}, nil, 0, math.MaxUint64, blobstore, NewMockMetricsService(), logger.Log)
			Expect(e).NotTo(HaveOccurred())

			reader, e := zip.OpenReader(tempFileName)
			Expect(e).NotTo(HaveOccurred())
			Expect(reader.File).To(HaveLen(1))
			lastModifedFromTempFile = reader.File[0].FileHeader.Modified
		})

		It("should not contain '1979-11-30'", func() {
			Expect(dateFormatter(lastModifedFromTempFile)).NotTo(ContainSubstring("1979-11-30"))
		})

		It("should provide the current timestamp for files retrieved from the bundles_cache", func() {
			d := time.Since(lastModifedFromTempFile)
			Expect(d.Seconds()).Should(BeNumerically("<", 2), "We accept difference from 2s for cached files.")
		})

		It("should not be manipulated for uploaded files", func() {
			tmpfile, modTime := createTmpFile()
			defer os.Remove(tmpfile)

			tmpfilereader, e := os.Open(tmpfile)
			Expect(e).NotTo(HaveOccurred())

			response := blobstore.Put("abc", tmpfilereader)
			Expect(response).To(Succeed())

			tempFileName, e := bitsgo.CreateTempZipFileFrom([]bitsgo.Fingerprint{
				bitsgo.Fingerprint{
					Sha1: "abc",
					Fn:   "filename1",
					Mode: "644",
				},
			}, nil, 0, math.MaxUint64, blobstore, NewMockMetricsService(), logger.Log)
			Expect(e).NotTo(HaveOccurred())

			reader, e := zip.OpenReader(tempFileName)
			Expect(e).NotTo(HaveOccurred())

			lm := reader.File[0].FileHeader.Modified
			Expect(dateFormatter(lm)).To(Equal(dateFormatter(modTime)))
		})

	})

	Context("One error from blobstore", func() {
		var blobstore *MockNoRedirectBlobstore

		BeforeEach(func() {
			blobstore = NewMockNoRedirectBlobstore()
		})

		Context("Error in Blobstore.Get", func() {
			It("Retries and creates the zip successfully", func() {
				When(blobstore.Get("abc")).
					ThenReturn(nil, errors.New("Some error")).
					ThenReturn(ioutil.NopCloser(strings.NewReader("filename1 content")), nil)

				tempFileName, e := bitsgo.CreateTempZipFileFrom([]bitsgo.Fingerprint{
					bitsgo.Fingerprint{
						Sha1: "abc",
						Fn:   "filename1",
						Mode: "644",
					},
				}, nil, 0, math.MaxUint64, blobstore, NewMockMetricsService(), logger.Log)
				Expect(e).NotTo(HaveOccurred())

				reader, e := zip.OpenReader(tempFileName)
				Expect(e).NotTo(HaveOccurred())
				Expect(reader.File).To(HaveLen(1))
				VerifyZipFileEntry(&reader.Reader, "filename1", "filename1 content")
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

				tempFileName, e := bitsgo.CreateTempZipFileFrom([]bitsgo.Fingerprint{
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
				}, nil, 0, math.MaxUint64, blobstore, NewMockMetricsService(), logger.Log)
				Expect(e).NotTo(HaveOccurred())

				reader, e := zip.OpenReader(tempFileName)
				Expect(e).NotTo(HaveOccurred())
				Expect(reader.File).To(HaveLen(2))
				VerifyZipFileEntry(&reader.Reader, "filename1", "filename1 content")
				VerifyZipFileEntry(&reader.Reader, "filename2", "filename2 content")
			})
		})
	})

	Context("maximumSize and minimumSize provided", func() {
		It("only stores the file which is within range of thresholds", func() {
			_, filename, _, _ := runtime.Caller(0)

			zipFile, e := os.Open(filepath.Join(filepath.Dir(filename), "assets", "test-file.zip"))
			Expect(e).NotTo(HaveOccurred())
			defer zipFile.Close()

			openZipFile, e := zip.OpenReader(zipFile.Name())
			Expect(e).NotTo(HaveOccurred())
			defer openZipFile.Close()

			tempFilename, e := bitsgo.CreateTempZipFileFrom([]bitsgo.Fingerprint{}, &openZipFile.Reader, 15, 30, blobstore, NewMockMetricsService(), logger.Log)
			Expect(e).NotTo(HaveOccurred())
			os.Remove(tempFilename)

			Expect(blobstore.Entries).To(HaveLen(1))
			Expect(blobstore.Entries).To(HaveKeyWithValue("e04c62ab0e87c29f862ee7c4e85c9fed51531dae", []byte("folder file content\n")))
		})
	})

	Context("More files in zip than ulimit allows per process", func() {
		It("does not fail with 'too many open files", func() {
			_, filename, _, _ := runtime.Caller(0)

			openZipFile, e := zip.OpenReader(filepath.Join(filepath.Dir(filename), "assets", "lots-of-files.zip"))
			Expect(e).NotTo(HaveOccurred())
			defer openZipFile.Close()

			tempFilename, e := bitsgo.CreateTempZipFileFrom([]bitsgo.Fingerprint{}, &openZipFile.Reader, 15, 30, blobstore, NewMockMetricsService(), logger.Log)
			Expect(e).NotTo(HaveOccurred(), "Error: %v", e)
			os.Remove(tempFilename)
		})
	})
})

func createTmpFile() (string, time.Time) {
	tmpfile, e := ioutil.TempFile("", "example")
	Expect(e).NotTo(HaveOccurred())
	_, e = tmpfile.Write([]byte("filename1 content"))
	Expect(e).NotTo(HaveOccurred())
	fileInfo, e := tmpfile.Stat()
	Expect(e).NotTo(HaveOccurred())
	Expect(tmpfile.Close()).To(Succeed())
	return tmpfile.Name(), fileInfo.ModTime()
}

func dateFormatter(anyTimeFormat time.Time) string {
	return anyTimeFormat.UTC().Format(time.UnixDate)
}
