package bitsgo

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/cenkalti/backoff"
)

func CreateTempZipFileFrom(bundlesPayload []Fingerprint,
	zipReader *zip.Reader,
	minimumSize, maximumSize uint64,
	blobstore NoRedirectBlobstore,
) (tempFilename string, err error) {
	tempFile, e := ioutil.TempFile("", "bundles")
	if e != nil {
		return "", e
	}
	defer tempFile.Close()
	zipWriter := zip.NewWriter(tempFile)

	if zipReader != nil {
		for _, zipInputFileEntry := range zipReader.File {
			if !zipInputFileEntry.FileInfo().Mode().IsRegular() {
				continue
			}
			zipFileEntryWriter, e := zipWriter.CreateHeader(zipEntryHeader(zipInputFileEntry.Name, zipInputFileEntry.FileInfo().Mode()))
			if e != nil {
				return "", e
			}
			zipEntryReader, e := zipInputFileEntry.Open()
			if e != nil {
				return "", e
			}
			defer zipEntryReader.Close()

			tempFile, e := ioutil.TempFile("", "app-stash")
			if e != nil {
				return "", e
			}
			defer os.Remove(tempFile.Name())
			defer tempFile.Close()

			sha := sha1.New()
			tempFileSize, e := io.Copy(io.MultiWriter(zipFileEntryWriter, tempFile, sha), zipEntryReader)
			if e != nil {
				return "", e
			}
			e = tempFile.Close()
			if e != nil {
				return "", e
			}
			e = zipEntryReader.Close()
			if e != nil {
				return "", e
			}
			tempFile, e = os.Open(tempFile.Name())
			if e != nil {
				return "", e
			}
			defer tempFile.Close()
			if uint64(tempFileSize) >= minimumSize && uint64(tempFileSize) <= maximumSize {
				e = blobstore.Put(hex.EncodeToString(sha.Sum(nil)), tempFile)
				if e != nil {
					return "", e
				}
			}
		}
	}

	for _, entry := range bundlesPayload {
		zipEntry, e := zipWriter.CreateHeader(zipEntryHeader(entry.Fn, fileModeFrom(entry.Mode)))
		if e != nil {
			return "", e
		}

		e = backoff.Retry(func() error {
			b, e := blobstore.Get(entry.Sha1)
			if e != nil {
				if _, ok := e.(*NotFoundError); ok {
					return backoff.Permanent(NewNotFoundErrorWithMessage(entry.Sha1))
				}
				return e
			}
			defer b.Close()

			_, e = io.Copy(zipEntry, b)
			if e != nil {
				return e
			}
			return nil
		},
			backoff.NewExponentialBackOff(),
		)
		if e != nil {
			return "", e
		}
	}
	zipWriter.Close()
	return tempFile.Name(), nil
}

func fileModeFrom(s string) os.FileMode {
	mode, e := strconv.ParseInt(s, 8, 32)
	if e != nil {
		return 0744
	}
	return os.FileMode(mode)
}

func zipEntryHeader(name string, mode os.FileMode) *zip.FileHeader {
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	header.SetMode(mode)
	return header
}
