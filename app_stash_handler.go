package bitsgo

import (
	"archive/zip"
	"crypto/sha1"
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

	"github.com/cloudfoundry-incubator/bits-service/logger"
	"github.com/cloudfoundry-incubator/bits-service/util"
	"github.com/pkg/errors"
)

type AppStashHandler struct {
	blobstore        Blobstore
	maxBodySizeLimit uint64
	minimumSize      uint64
	maximumSize      uint64
	metricsService   MetricsService
}

func NewAppStashHandlerWithSizeThresholds(blobstore Blobstore, maxBodySizeLimit uint64, minimumSize uint64, maximumSize uint64, metricsService MetricsService) *AppStashHandler {
	return &AppStashHandler{
		blobstore:        blobstore,
		maxBodySizeLimit: maxBodySizeLimit,
		minimumSize:      minimumSize,
		maximumSize:      maximumSize,
		metricsService:   metricsService,
	}
}

func (handler *AppStashHandler) PostMatches(responseWriter http.ResponseWriter, request *http.Request) {
	if !HandleBodySizeLimits(responseWriter, request, handler.maxBodySizeLimit) {
		return
	}
	body, e := ioutil.ReadAll(request.Body)
	util.PanicOnError(e)
	var fingerprints []Fingerprint
	e = json.Unmarshal(body, &fingerprints)
	if e != nil {
		logger.From(request).Debugw("Invalid body", "error", e, "body", string(body))
		responseWriter.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(responseWriter, "Invalid body %s", body)
		return
	}
	if len(fingerprints) == 0 {
		logger.From(request).Debugw("Empty list", "error", e, "body", string(body))
		responseWriter.WriteHeader(http.StatusUnprocessableEntity)
		util.FprintDescriptionAsJSON(responseWriter, "The request is semantically invalid: must be a non-empty array.")
		return
	}
	matchedFingerprints := []Fingerprint{} // this must not be nil, because the JSON marshaller will not marshal it correctly in case of []
	for _, entry := range fingerprints {
		if entry.Size < handler.minimumSize || entry.Size > handler.maximumSize {
			continue
		}
		exists, e := handler.blobstore.Exists(entry.Sha1)
		util.PanicOnError(e)
		if exists {
			matchedFingerprints = append(matchedFingerprints, entry)
		}
	}
	response, e := json.Marshal(&matchedFingerprints)
	util.PanicOnError(e)
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
	util.PanicOnError(e)
	defer os.Remove(tempZipFile.Name())
	defer tempZipFile.Close()

	_, e = io.Copy(tempZipFile, uploadedFile)
	util.PanicOnError(e)

	openZipFile, e := zip.OpenReader(tempZipFile.Name())
	if e != nil {
		badRequest(responseWriter, request, "Bad Request: Not a valid zip file")
		return
	}
	defer openZipFile.Close()

	fingerprints := []Fingerprint{} // this must not be nil, because the JSON marshaller will not marshal it correctly in case of []
	for _, zipFileEntry := range openZipFile.File {
		if !zipFileEntry.FileInfo().Mode().IsRegular() {
			continue
		}
		sha, e := copyTo(handler.blobstore, zipFileEntry)
		if _, isNoSpaceLeftError := e.(*NoSpaceLeftError); isNoSpaceLeftError {
			http.Error(responseWriter, util.DescriptionAndCodeAsJSON(500000, "Request Entity Too Large"), http.StatusInsufficientStorage)
			return
		}
		util.PanicOnError(e)
		logger.From(request).Debugw("Filemode in zip File Entry", "filemode", zipFileEntry.FileInfo().Mode().String())
		fingerprints = append(fingerprints, Fingerprint{
			Sha1: sha,
			Fn:   zipFileEntry.Name,
			Mode: strconv.FormatInt(int64(zipFileEntry.FileInfo().Mode()), 8),
			Size: zipFileEntry.UncompressedSize64,
		})
	}
	receipt, e := json.Marshal(fingerprints)
	util.PanicOnError(e)
	responseWriter.WriteHeader(http.StatusCreated)
	responseWriter.Write(receipt)
}

func copyTo(blobstore Blobstore, zipFileEntry *zip.File) (sha string, err error) {
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

type Fingerprint struct {
	Fn   string `json:"fn"`
	Sha1 string `json:"sha1"`
	Size uint64 `json:"size"`
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
		util.PanicOnError(e)
		zipFile, fi, e := request.FormFile("application")
		if e == http.ErrMissingFile {
			badRequest(responseWriter, request, "Could not retrieve form parameter 'application")
			return
		}
		util.PanicOnError(e)
		defer zipFile.Close()
		zipReader, e = zip.NewReader(zipFile, fi.Size)
		util.PanicOnError(e)
	} else {
		resources = request.Body
	}

	body, e := ioutil.ReadAll(resources)
	util.PanicOnError(e)

	var bundlesPayload []Fingerprint
	e = json.Unmarshal(body, &bundlesPayload)
	if e != nil {
		log.Printf("Invalid body %s", body)
		responseWriter.WriteHeader(http.StatusUnprocessableEntity)
		util.FprintDescriptionAsJSON(responseWriter, "Invalid body %s", body)
		return
	}

	if isMissing, key := anyKeyMissingIn(bundlesPayload); isMissing {
		responseWriter.WriteHeader(http.StatusUnprocessableEntity)
		util.FprintDescriptionAsJSON(responseWriter, "The request is semantically invalid: key `%v` missing or empty", key)
		return
	}

	tempZipFilename, e := CreateTempZipFileFrom(bundlesPayload, zipReader, handler.minimumSize, handler.maximumSize, handler.blobstore, handler.metricsService, logger.From(request))
	if e != nil {
		if notFoundError, ok := e.(*NotFoundError); ok {
			responseWriter.WriteHeader(http.StatusNotFound)
			util.FprintDescriptionAsJSON(responseWriter, "%v not found", notFoundError.MissingKey)
			return
		}
		panic(e)
	}
	defer os.Remove(tempZipFilename)

	tempZipFile, e := os.Open(tempZipFilename)
	util.PanicOnError(e)
	defer tempZipFile.Close()

	_, e = io.Copy(responseWriter, tempZipFile)
	util.PanicOnError(e)
}

func anyKeyMissingIn(bundlesPayload []Fingerprint) (bool, string) {
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
