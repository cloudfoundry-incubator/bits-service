package bitsgo

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/cenkalti/backoff"
)

func CreateTempZipFileFrom(bundlesPayload []Fingerprint,
	zipReader *zip.Reader,
	minimumSize, maximumSize uint64,
	blobstore NoRedirectBlobstore,
	metricsService MetricsService,
) (tempFilename string, err error) {
	tempZipFile, e := ioutil.TempFile("", "bundles")
	if e != nil {
		return "", errors.Wrap(e, "Could not create temp file")
	}
	defer func() {
		if err != nil {
			os.Remove(tempZipFile.Name())
		}
	}()
	defer tempZipFile.Close()
	zipWriter := zip.NewWriter(tempZipFile)

	if zipReader != nil {
		for _, zipInputFileEntry := range zipReader.File {
			if !zipInputFileEntry.FileInfo().Mode().IsRegular() {
				continue
			}
			zipFileEntryWriter, e := zipWriter.CreateHeader(zipEntryHeaderWithModifiedTime(zipInputFileEntry.Name, zipInputFileEntry.FileInfo().Mode(), zipInputFileEntry.FileHeader.Modified))
			if e != nil {
				return "", errors.Wrap(e, "Could not create header in zip file")
			}
			zipEntryReader, e := zipInputFileEntry.Open()
			if e != nil {
				return "", errors.Wrap(e, "Could not open zip file entry")
			}
			defer zipEntryReader.Close()

			tempFile, e := ioutil.TempFile("", "app-stash")
			if e != nil {
				return "", errors.Wrap(e, "Could not create tempfile")
			}
			defer os.Remove(tempFile.Name())
			defer tempFile.Close()

			sha := sha1.New()
			tempFileSize, e := io.Copy(io.MultiWriter(zipFileEntryWriter, tempFile, sha), zipEntryReader)
			if e != nil {
				return "", errors.Wrap(e, "Could not copy content from zip entry")
			}
			e = tempFile.Close()
			if e != nil {
				return "", errors.Wrap(e, "Could not close temp file")
			}
			e = zipEntryReader.Close()
			if e != nil {
				return "", errors.Wrap(e, "Could not close zip entry reader")
			}
			e = backoff.RetryNotify(func() error {
				tempFile, e = os.Open(tempFile.Name())
				if e != nil {
					return errors.Wrap(e, "Could not open temp file for reading")
				}
				defer tempFile.Close()
				if uint64(tempFileSize) >= minimumSize && uint64(tempFileSize) <= maximumSize {
					sha := hex.EncodeToString(sha.Sum(nil))
					e = blobstore.Put(sha, tempFile)
					if e != nil {
						if _, ok := e.(*NoSpaceLeftError); ok {
							return backoff.Permanent(e)
						}
						return errors.Wrapf(e, "Could not upload file to blobstore. SHA: '%v'", sha)
					}
				}
				return nil
			}, backoff.NewExponentialBackOff(), func(e error, backOffDelay time.Duration) {
				metricsService.SendCounterMetric("appStashPutRetries", 1)
			})
			if e != nil {
				return "", e
			}
			os.Remove(tempFile.Name())
		}
	}

	for _, entry := range bundlesPayload {
		zipEntry, e := zipWriter.CreateHeader(zipEntryHeaderWithModifiedTime(entry.Fn, fileModeFrom(entry.Mode), time.Now()))
		if e != nil {
			return "", errors.Wrap(e, "Could create header in zip file")
		}

		e = backoff.RetryNotify(func() error {
			b, e := blobstore.Get(entry.Sha1)

			if e != nil {
				if _, ok := e.(*NotFoundError); ok {
					return backoff.Permanent(NewNotFoundErrorWithKey(entry.Sha1))
				}
				return errors.Wrapf(e, "Could not get file from blobstore. SHA: '%v'", entry.Sha1)
			}
			defer b.Close()

			_, e = io.Copy(zipEntry, b)
			if e != nil {
				return errors.Wrapf(e, "Could not copy file to zip entry. SHA: %v", entry.Sha1)
			}
			return nil
		},
			backoff.NewExponentialBackOff(),
			func(e error, backOffDelay time.Duration) {
				metricsService.SendCounterMetric("appStashGetRetries", 1)
			},
		)
		if e != nil {
			return "", e
		}
	}
	zipWriter.Close()
	return tempZipFile.Name(), nil
}

func fileModeFrom(s string) os.FileMode {
	mode, e := strconv.ParseInt(s, 8, 32)
	if e != nil {
		return 0744
	}
	return os.FileMode(mode)
}

func zipEntryHeaderWithModifiedTime(name string, mode os.FileMode, modified time.Time) *zip.FileHeader {
	header := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: modified,
	}
	header.SetMode(mode)
	return header
}
