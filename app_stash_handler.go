package bitsgo

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cenkalti/backoff"
	"github.com/petergtz/bitsgo/logger"
	"github.com/pkg/errors"
)

type AppStashHandler struct {
	blobstore        NoRedirectBlobstore
	maxBodySizeLimit uint64
}

func NewAppStashHandler(blobstore NoRedirectBlobstore, maxBodySizeLimit uint64) *AppStashHandler {
	return &AppStashHandler{
		blobstore:        blobstore,
		maxBodySizeLimit: maxBodySizeLimit,
	}
}

func (handler *AppStashHandler) PostMatches(responseWriter http.ResponseWriter, request *http.Request) {
	if !HandleBodySizeLimits(responseWriter, request, handler.maxBodySizeLimit) {
		return
	}
	body, e := ioutil.ReadAll(request.Body)
	if e != nil {
		internalServerError(responseWriter, request, e)
		return
	}
	var sha1s []struct {
		Sha1 string
		Size int
	}
	e = json.Unmarshal(body, &sha1s)
	if e != nil {
		logger.From(request).Debugw("Invalid body", "body", string(body), "error", e)
		responseWriter.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(responseWriter, "Invalid body %s", body)
		return
	}
	if len(sha1s) == 0 {
		logger.From(request).Debugw("Empty list", "body", string(body), "error", e)
		responseWriter.WriteHeader(http.StatusUnprocessableEntity)
		fprintDescriptionAsJSON(responseWriter, "The request is semantically invalid: must be a non-empty array.")
		return
	}
	responseSha1 := []map[string]string{}
	for _, entry := range sha1s {
		exists, e := handler.blobstore.Exists(entry.Sha1)
		if e != nil {
			internalServerError(responseWriter, request, e)
			return
		}
		if exists {
			responseSha1 = append(responseSha1, map[string]string{"sha1": entry.Sha1})
		}
	}
	response, e := json.Marshal(&responseSha1)
	if e != nil {
		internalServerError(responseWriter, request, e)
		return
	}
	responseWriter.Write(response)
}

func (handler *AppStashHandler) PostEntries(responseWriter http.ResponseWriter, request *http.Request) {
	if !HandleBodySizeLimits(responseWriter, request, handler.maxBodySizeLimit) {
		return
	}
	uploadedFile, _, e := request.FormFile("application")
	if e != nil {
		badRequest(responseWriter, request, "Could not retrieve 'application' form parameter")
		return
	}
	defer uploadedFile.Close()

	tempZipFile, e := ioutil.TempFile("", "")
	if e != nil {
		internalServerError(responseWriter, request, e)
		return
	}
	defer os.Remove(tempZipFile.Name())
	defer tempZipFile.Close()

	_, e = io.Copy(tempZipFile, uploadedFile)
	if e != nil {
		internalServerError(responseWriter, request, e)
		return
	}

	openZipFile, e := zip.OpenReader(tempZipFile.Name())
	if e != nil {
		badRequest(responseWriter, request, "Bad Request: Not a valid zip file")
		return
	}
	defer openZipFile.Close()

	bundlesPayload := []BundlesPayload{}
	for _, zipFileEntry := range openZipFile.File {
		if !zipFileEntry.FileInfo().Mode().IsRegular() {
			continue
		}
		sha, e := copyTo(handler.blobstore, zipFileEntry)
		if _, isNoSpaceLeftError := e.(*NoSpaceLeftError); isNoSpaceLeftError {
			http.Error(responseWriter, descriptionAndCodeAsJSON("500000", "Request Entity Too Large"), http.StatusInsufficientStorage)
			return
		}
		if e != nil {
			internalServerError(responseWriter, request, e)
			return
		}
		logger.From(request).Debugw("Filemode in zip File Entry", "filemode", zipFileEntry.FileInfo().Mode().String())
		bundlesPayload = append(bundlesPayload, BundlesPayload{
			Sha1: sha,
			Fn:   zipFileEntry.Name,
			Mode: strconv.FormatInt(int64(zipFileEntry.FileInfo().Mode()), 8),
		})
	}
	receipt, e := json.Marshal(bundlesPayload)
	if e != nil {
		internalServerError(responseWriter, request, e)
		return
	}
	responseWriter.WriteHeader(http.StatusCreated)
	responseWriter.Write(receipt)
}

func copyTo(blobstore NoRedirectBlobstore, zipFileEntry *zip.File) (sha string, err error) {
	unzippedReader, e := zipFileEntry.Open()
	if e != nil {
		return "", errors.WithStack(e)
	}
	defer unzippedReader.Close()

	tempZipEntryFile, e := ioutil.TempFile("", filepath.Base(zipFileEntry.Name))
	if e != nil {
		return "", errors.WithStack(e)
	}
	defer os.Remove(tempZipEntryFile.Name())
	defer tempZipEntryFile.Close()

	sha, e = copyCalculatingSha(tempZipEntryFile, unzippedReader)
	if e != nil {
		return "", errors.WithStack(e)
	}

	entryFileRead, e := os.Open(tempZipEntryFile.Name())
	if e != nil {
		return "", errors.WithStack(e)
	}
	defer entryFileRead.Close()

	e = blobstore.Put(sha, entryFileRead)
	if _, noSpaceLeft := e.(*NoSpaceLeftError); noSpaceLeft {
		return "", e
	}
	if e != nil {
		return "", errors.WithStack(e)
	}

	return
}

func copyCalculatingSha(writer io.Writer, reader io.Reader) (sha string, e error) {
	checkSum := sha1.New()

	_, e = io.Copy(io.MultiWriter(writer, checkSum), reader)
	if e != nil {
		return "", fmt.Errorf("error copying. Caused by: %v", e)
	}

	return fmt.Sprintf("%x", checkSum.Sum(nil)), nil
}

type BundlesPayload struct {
	Sha1 string `json:"sha1"`
	Fn   string `json:"fn"`
	Mode string `json:"mode"`
}

func (handler *AppStashHandler) PostBundles(responseWriter http.ResponseWriter, request *http.Request) {
	if !HandleBodySizeLimits(responseWriter, request, handler.maxBodySizeLimit) {
		return
	}

	var (
		resources io.Reader
		e         error
	)
	var zipReader *zip.Reader
	if strings.Contains(request.Header.Get("Content-Type"), "multipart/form-data") {
		resources, _, e = request.FormFile("resources")
		if e == http.ErrMissingFile {
			badRequest(responseWriter, request, "Could not retrieve form parameter 'resources")
			return
		}
		if e != nil {
			internalServerError(responseWriter, request, e)
			return
		}
		zipFile, fi, e := request.FormFile("application")
		if e == http.ErrMissingFile {
			badRequest(responseWriter, request, "Could not retrieve form parameter 'application")
			return
		}
		if e != nil {
			internalServerError(responseWriter, request, e)
			return
		}
		defer zipFile.Close()
		zipReader, e = zip.NewReader(zipFile, fi.Size)
		if e != nil {
			internalServerError(responseWriter, request, e)
			return
		}
	} else {
		resources = request.Body
	}

	body, e := ioutil.ReadAll(resources)
	if e != nil {
		internalServerError(responseWriter, request, e)
		return
	}

	var bundlesPayload []BundlesPayload
	e = json.Unmarshal(body, &bundlesPayload)
	if e != nil {
		log.Printf("Invalid body %s", body)
		responseWriter.WriteHeader(http.StatusUnprocessableEntity)
		fprintDescriptionAsJSON(responseWriter, "Invalid body %s", body)
		return
	}

	if isMissing, key := anyKeyMissingIn(bundlesPayload); isMissing {
		responseWriter.WriteHeader(http.StatusUnprocessableEntity)
		fprintDescriptionAsJSON(responseWriter, "The request is semantically invalid: key `%v` missing or empty", key)
		return
	}

	tempZipFilename, e := handler.CreateTempZipFileFrom(bundlesPayload, zipReader)
	if e != nil {
		if notFoundError, ok := e.(*NotFoundError); ok {
			responseWriter.WriteHeader(http.StatusNotFound)
			fprintDescriptionAsJSON(responseWriter, "%v not found", notFoundError.Error())
			return
		}
		internalServerError(responseWriter, request, e)
		return
	}
	defer os.Remove(tempZipFilename)

	tempZipFile, e := os.Open(tempZipFilename)
	if e != nil {
		internalServerError(responseWriter, request, e)
		return
	}
	defer tempZipFile.Close()

	_, e = io.Copy(responseWriter, tempZipFile)
	if e != nil {
		internalServerError(responseWriter, request, e)
		return
	}
}

func fprintDescriptionAsJSON(responseWriter http.ResponseWriter, description string, a ...interface{}) {
	fmt.Fprintf(responseWriter, `{"description":"%v"}`, fmt.Sprintf(description, a...))
}

func anyKeyMissingIn(bundlesPayload []BundlesPayload) (bool, string) {
	for _, entry := range bundlesPayload {
		if entry.Sha1 == "" {
			return true, "sha1"
		}
		if entry.Fn == "" {
			return true, "fn"
		}
	}
	return false, ""
}

func (handler *AppStashHandler) CreateTempZipFileFrom(bundlesPayload []BundlesPayload, zipReader *zip.Reader) (tempFilename string, err error) {
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
			_, e = io.Copy(io.MultiWriter(zipFileEntryWriter, tempFile, sha), zipEntryReader)
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
			e = handler.blobstore.Put(hex.EncodeToString(sha.Sum(nil)), tempFile)
			if e != nil {
				return "", e
			}
		}
	}

	for _, entry := range bundlesPayload {
		zipEntry, e := zipWriter.CreateHeader(zipEntryHeader(entry.Fn, fileModeFrom(entry.Mode)))
		if e != nil {
			return "", e
		}

		e = backoff.Retry(func() error {
			b, e := handler.blobstore.Get(entry.Sha1)
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
