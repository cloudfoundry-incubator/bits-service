package bitsgo_test

import (
	"archive/zip"
	"errors"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo"
	inmemory "github.com/petergtz/bitsgo/blobstores/inmemory"
	. "github.com/petergtz/bitsgo/matchers"
	. "github.com/petergtz/bitsgo/testutil"
	. "github.com/petergtz/pegomock"
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
		}, nil, 0, math.MaxUint64, blobstore)
		Expect(e).NotTo(HaveOccurred())

		reader, e := zip.OpenReader(tempFileName)
		Expect(e).NotTo(HaveOccurred())
		Expect(reader.File).To(HaveLen(1))
		VerifyZipFileEntry(&reader.Reader, "filename1", "filename1 content")
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
				}, nil, 0, math.MaxUint64, blobstore)
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
				}, nil, 0, math.MaxUint64, blobstore)
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

			tempFilename, e := bitsgo.CreateTempZipFileFrom([]bitsgo.Fingerprint{}, &openZipFile.Reader, 15, 30, blobstore)
			Expect(e).NotTo(HaveOccurred())
			os.Remove(tempFilename)

			Expect(blobstore.Entries).To(HaveLen(1))
			Expect(blobstore.Entries).To(HaveKeyWithValue("e04c62ab0e87c29f862ee7c4e85c9fed51531dae", []byte("folder file content\n")))
		})
	})
})
