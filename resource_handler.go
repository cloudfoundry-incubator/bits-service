package bitsgo

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"go.uber.org/zap"

	"github.com/cenkalti/backoff"
	"github.com/cloudfoundry-incubator/bits-service/logger"
	"github.com/cloudfoundry-incubator/bits-service/util"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

type StateForbiddenError struct {
	error
}

func NewStateForbiddenError() *StateForbiddenError {
	return &StateForbiddenError{fmt.Errorf("StateForbiddenError")}
}

type Updater interface {
	NotifyProcessingUpload(guid string) error
	NotifyUploadSucceeded(guid string, sha1 string, sha2 string) error
	NotifyUploadFailed(guid string, e error) error
}

type NullUpdater struct{}

func (u *NullUpdater) NotifyProcessingUpload(guid string) error                          { return nil }
func (u *NullUpdater) NotifyUploadSucceeded(guid string, sha1 string, sha2 string) error { return nil }
func (u *NullUpdater) NotifyUploadFailed(guid string, e error) error                     { return nil }

type ResourceHandler struct {
	blobstore              Blobstore
	appStashBlobstore      Blobstore
	resourceType           string
	metricsService         MetricsService
	maxBodySizeLimit       uint64
	updater                Updater
	minimumSize            uint64
	maximumSize            uint64
	shouldProxyGetRequests bool
}

type ResponseBody struct {
	Guid      string    `json:"guid"`
	State     string    `json:"state"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Sha1      string    `json:"sha1"`
	Sha256    string    `json:"sha256"`
}

func NewResourceHandler(blobstore Blobstore, appStashBlobstore Blobstore, resourceType string, metricsService MetricsService, maxBodySizeLimit uint64, shouldProxyGetRequests bool) *ResourceHandler {
	return NewResourceHandlerWithUpdater(
		blobstore,
		appStashBlobstore,
		&NullUpdater{},
		resourceType,
		metricsService,
		maxBodySizeLimit,
		shouldProxyGetRequests,
	)
}

func NewResourceHandlerWithUpdater(blobstore Blobstore, appStashBlobstore Blobstore, updater Updater, resourceType string, metricsService MetricsService, maxBodySizeLimit uint64, shouldProxyGetRequests bool) *ResourceHandler {
	return NewResourceHandlerWithUpdaterAndSizeThresholds(
		blobstore,
		appStashBlobstore,
		updater,
		resourceType,
		metricsService,
		maxBodySizeLimit,
		0, math.MaxUint64,
		shouldProxyGetRequests,
	)
}

func NewResourceHandlerWithUpdaterAndSizeThresholds(blobstore Blobstore, appStashBlobstore Blobstore, updater Updater, resourceType string, metricsService MetricsService, maxBodySizeLimit uint64, minimumSize, maximumSize uint64, shouldProxyGetRequests bool) *ResourceHandler {
	return &ResourceHandler{
		blobstore:              blobstore,
		appStashBlobstore:      appStashBlobstore,
		resourceType:           resourceType,
		metricsService:         metricsService,
		maxBodySizeLimit:       maxBodySizeLimit,
		updater:                updater,
		maximumSize:            maximumSize,
		minimumSize:            minimumSize,
		shouldProxyGetRequests: shouldProxyGetRequests,
	}
}

// TODO: instead of params, we could use `identifier string` to make the interface more type-safe.
//       Here and in the other methods.
func (handler *ResourceHandler) AddOrReplaceWithDigestInHeader(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	if !HandleBodySizeLimits(responseWriter, request, handler.maxBodySizeLimit) {
		return
	}
	logger.From(request).Debugw("Octet-Stream")

	digest := request.Header.Get("Digest")
	if digest == "" {
		badRequest(responseWriter, request, "No Digest header")
		return
	}
	parts := strings.Split(digest, "=")
	if len(parts) != 2 {
		badRequest(responseWriter, request, "Digest must have format sha256=value, but is "+digest)
		return
	}
	alg, value := strings.ToLower(parts[0]), parts[1]
	if alg != "sha256" {
		badRequest(responseWriter, request, "Digest must have format sha256=value, but is "+digest)
		return
	}
	if value == "" {
		badRequest(responseWriter, request, "Digest must have format sha256=value. Value cannot be empty")
		return
	}

	// TODO this can cause an out of memory panic. Should be smart about writing big files to disk instead.
	content, e := ioutil.ReadAll(request.Body)
	util.PanicOnError(e)

	e = backoff.RetryNotify(func() error {
		e := handler.blobstore.Put(params["identifier"]+"/"+value, bytes.NewReader(content))
		if e != nil {
			if _, noSpaceLeft := e.(*NoSpaceLeftError); noSpaceLeft {
				return backoff.Permanent(e)
			}
			return errors.Wrap(e, "Could not upload bits to blobstore")
		}
		return nil

	}, retryPolicy(), func(e error, delay time.Duration) {
		handler.metricsService.SendCounterMetric("upload"+handler.resourceType, 1)
	})

	// TODO use Clock instead:
	writeResponseBasedOn("", e, responseWriter, request, http.StatusCreated, nil, &ResponseBody{Guid: params["identifier"], State: "READY", Type: "bits", CreatedAt: time.Now()}, "")
}

// TODO: instead of params, we could use `identifier string` to make the interface more type-safe.
//       Here and in the other methods.
func (handler *ResourceHandler) AddOrReplace(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	if !HandleBodySizeLimits(responseWriter, request, handler.maxBodySizeLimit) {
		return
	}
	file, fileInfo, e := request.FormFile(handler.resourceType)
	if e == http.ErrMissingFile {
		file, fileInfo, e = request.FormFile("bits")
	}
	if e == http.ErrMissingFile {
		badRequest(responseWriter, request, "Could not retrieve form parameter '%s' or 'bits", handler.resourceType)
		return
	}
	util.PanicOnError(e)
	defer file.Close()

	var tempFilename string
	// TODO: this if-block maybe not be necessary at all.
	//       The reason it's necessary right now is that we need zip handling only for packages. We treat other resources opaque.
	if handler.resourceType == "package" {
		tempFilename, e = handler.completePackageWithResources(request.FormValue("resources"), file, fileInfo.Size, logger.From(request))
		switch e.(type) {
		case *inputError:
			logger.From(request).Infow(e.Error())
			responseWriter.WriteHeader(http.StatusUnprocessableEntity)
			util.FprintDescriptionAsJSON(responseWriter, e.Error())
			return
		case *NoSpaceLeftError:
			http.Error(responseWriter, util.DescriptionAndCodeAsJSON(500000, "Request Entity Too Large"), http.StatusInsufficientStorage)
			return
		case error:
			panic(e)
		}
	} else {
		tempFilename, e = CreateTempFileWithContent(file)
		util.PanicOnError(e)
	}

	sha1, sha256, e := ShaSums(tempFilename)
	util.PanicOnError(e)

	e = handler.updater.NotifyProcessingUpload(params["identifier"])
	if handleNotificationError(e, responseWriter, request) {
		return
	}

	if request.URL.Query().Get("async") == "true" {
		go handler.uploadResource(tempFilename, request, params["identifier"], true, sha1, sha256)
		writeResponseBasedOn("", nil, responseWriter, request, http.StatusAccepted, nil, &ResponseBody{
			Guid:      params["identifier"],
			State:     "PROCESSING_UPLOAD",
			Type:      "bits",
			CreatedAt: time.Now(),
			Sha1:      hex.EncodeToString(sha1),
			Sha256:    hex.EncodeToString(sha256),
		}, "")
	} else {
		e = handler.uploadResource(tempFilename, request, params["identifier"], false, sha1, sha256)
		if IsNotFoundError(e) {
			writeResponseBasedOn("", nil, responseWriter, request, http.StatusConflict, nil, nil, "")
			return
		}
		writeResponseBasedOn("", e, responseWriter, request, http.StatusCreated, nil, &ResponseBody{
			Guid:      params["identifier"],
			State:     "READY",
			Type:      "bits",
			CreatedAt: time.Now(),
			Sha1:      hex.EncodeToString(sha1),
			Sha256:    hex.EncodeToString(sha256),
		}, "")
	}
}

type BuildpackMetadata struct {
	Filename string `json:"filename"`
	Sha1     string `json:"sha1"`
	Sha256   string `json:"sha256"`
	Stack    string `json:"stack"`
	Key      string `json:"key"`
}

func (handler *ResourceHandler) AddBuildpack(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	if !HandleBodySizeLimits(responseWriter, request, handler.maxBodySizeLimit) {
		return
	}
	file, fileInfo, e := request.FormFile(handler.resourceType)
	if e == http.ErrMissingFile {
		file, fileInfo, e = request.FormFile("bits")
	}
	if e == http.ErrMissingFile {
		badRequest(responseWriter, request, "Could not retrieve form parameter '%s' or 'bits", handler.resourceType)
		return
	}
	util.PanicOnError(e)
	defer file.Close()

	tempFilename, e := CreateTempFileWithContent(file)
	util.PanicOnError(e)

	sha1, sha256, e := ShaSums(tempFilename)
	util.PanicOnError(e)

	stack, e := extractStackFromZipFile(tempFilename)
	if e != nil {
		badRequest(responseWriter, request, "Invalid buildpack zip file: %v", e.Error())
		return
	}

	// TODO (pego, eli): do we want to re-introduce async upload here?

	identifier := uuid.NewV4().String()

	e = handler.uploadResource(tempFilename, request, identifier, false, sha1, sha256)
	util.PanicOnError(e)

	buildpackMetadata := BuildpackMetadata{
		Filename: fileInfo.Filename,
		Sha1:     hex.EncodeToString(sha1),
		Sha256:   hex.EncodeToString(sha256),
		Stack:    stack,
		Key:      identifier,
	}

	bpMetadataJson, e := json.Marshal(buildpackMetadata)
	util.PanicOnError(e)
	e = handler.blobstore.Put(identifier+"-metadata", bytes.NewReader(bpMetadataJson))
	util.PanicOnError(e)
	writeResponseBasedOn("", e, responseWriter, request, http.StatusCreated, nil, &ResponseBody{
		Guid:      buildpackMetadata.Key,
		State:     "READY",
		Type:      "bits",
		CreatedAt: time.Now(),
		Sha1:      buildpackMetadata.Sha1,
		Sha256:    buildpackMetadata.Sha256,
	}, "")
}

func extractStackFromZipFile(tempFilename string) (string, error) {
	buildpackFile, e := zip.OpenReader(tempFilename)
	switch e {
	case zip.ErrFormat, zip.ErrAlgorithm, zip.ErrChecksum:
		return "", e
	default:
		util.PanicOnError(e)
	}
	for _, zipEntry := range buildpackFile.File {
		if zipEntry.FileInfo().IsDir() || zipEntry.Name != "manifest.yml" {
			continue
		}
		manifestReader, e := zipEntry.Open()
		util.PanicOnError(e)

		manifestCOntent, e := ioutil.ReadAll(manifestReader)
		util.PanicOnError(e)

		var buildpackManifest struct {
			Stack string `yaml:"stack"`
		}
		e = yaml.Unmarshal(manifestCOntent, &buildpackManifest)
		if e != nil {
			return "", errors.New("manifest.yml is an invalid YAML file")
		}
		if buildpackManifest.Stack == "" {
			return "", errors.New("missing key \"stack\" in manifest.yml")
		}
		return buildpackManifest.Stack, nil
	}

	return "", errors.New("no manifest.yml found in zip")
}

type inputError struct {
	error
}

// returns inputError or NoSpaceLeftError in case of error
func (handler *ResourceHandler) completePackageWithResources(resources string, file multipart.File, fileSize int64, logger *zap.SugaredLogger) (tempfileName string, err error) {
	var bundlesPayload []Fingerprint
	if resources != "" {
		e := json.Unmarshal([]byte(resources), &bundlesPayload)
		if e != nil {
			return "", &inputError{fmt.Errorf("The request is semantically invalid: JSON payload could not be parsed: '%s'", resources)}
		}
		if isMissing, key := anyKeyMissingIn(bundlesPayload); isMissing {
			return "", &inputError{fmt.Errorf("The request is semantically invalid: key \"%v\" missing or empty", key)}
		}
	}
	zipReader, e := zip.NewReader(file, fileSize)
	if e != nil && strings.Contains(e.Error(), "not a valid zip file") {
		return "", &inputError{fmt.Errorf("The request is semantically invalid: bits uploaded is not a valid zip file")}
	}
	util.PanicOnError(e)

	tempFilename, e := CreateTempZipFileFrom(bundlesPayload, zipReader, handler.minimumSize, handler.maximumSize, handler.appStashBlobstore, handler.metricsService, logger)
	if _, noSpaceLeft := e.(*NoSpaceLeftError); noSpaceLeft {
		return "", e
	}
	if notFoundErr, ok := e.(*NotFoundError); ok {
		return "", &inputError{fmt.Errorf("The request is semantically invalid: not all specified sha1s could be found in app-stash. Missing sha1: \"%v\"", notFoundErr.MissingKey)}
	}
	util.PanicOnError(e)
	return tempFilename, nil
}

func CreateTempFileWithContent(reader io.Reader) (string, error) {
	uploadedFile, e := ioutil.TempFile("", "bits")
	if e != nil {
		return "", errors.WithStack(e)
	}
	_, e = io.Copy(uploadedFile, reader)
	if e != nil {
		uploadedFile.Close()
		return "", errors.WithStack(e)
	}
	uploadedFile.Close()

	return uploadedFile.Name(), nil
}

func (handler *ResourceHandler) uploadResource(tempFilename string, request *http.Request, identifier string, async bool, sha1Sum []byte, sha256Sum []byte) error {
	defer os.Remove(tempFilename)
	e := backoff.RetryNotify(func() error {
		tempFile, e := os.Open(tempFilename)
		if e != nil {
			return backoff.Permanent(errors.Wrapf(e, "Could not open temporary file '%v'", tempFilename))
		}
		defer tempFile.Close()

		logger.From(request).Debugw("Starting upload to blobstore", "identifier", identifier)
		e = handler.blobstore.Put(identifier, tempFile)
		logger.From(request).Debugw("Completed upload to blobstore", "identifier", identifier)

		if e != nil {
			if _, noSpaceLeft := e.(*NoSpaceLeftError); noSpaceLeft {
				return backoff.Permanent(e)
			}

			return errors.Wrapf(e, "Could not upload temporary file to blobstore %v", tempFilename)
		}
		return nil
	}, retryPolicy(), func(e error, delay time.Duration) {
		handler.metricsService.SendCounterMetric("upload"+handler.resourceType, 1)
	})

	if e != nil {
		handler.notifyUploadFailed(identifier, e, request)
		return handle(e, async, request)
	}
	e = handler.updater.NotifyUploadSucceeded(identifier, hex.EncodeToString(sha1Sum), hex.EncodeToString(sha256Sum))
	if IsNotFoundError(e) {
		return e
	}
	if e != nil {
		return handle(errors.Wrapf(e, "Could not notify Cloud Controller about successful upload"), async, request)
	}
	return nil
}

// TODO(pego): find better name for this function
func handle(e error, async bool, request *http.Request) error {
	if async {
		logger.From(request).Errorw("Failure during upload", "error", e)
	}
	return e
}

func retryPolicy() *backoff.ExponentialBackOff {
	retryPolicy := backoff.NewExponentialBackOff()
	retryPolicy.MaxElapsedTime = time.Second
	return retryPolicy
}

func (handler *ResourceHandler) notifyUploadFailed(identifier string, e error, request *http.Request) {
	notifyErr := handler.updater.NotifyUploadFailed(identifier, e)
	if notifyErr != nil {
		logger.From(request).Errorw("Failed to notifying CC about failed upload.", "error", notifyErr)
	}
}

func ShaSums(filename string) (sha1Sum []byte, sha256Sum []byte, e error) {
	file, e := os.Open(filename)
	if e != nil {
		return nil, nil, errors.WithStack(e)
	}
	defer file.Close()
	sha1Hash := sha1.New()
	sha256Hash := sha256.New()
	_, e = io.Copy(io.MultiWriter(sha1Hash, sha256Hash), file)
	if e != nil {
		return nil, nil, errors.WithStack(e)
	}
	return sha1Hash.Sum(nil), sha256Hash.Sum(nil), nil
}

func handleNotificationError(e error, responseWriter http.ResponseWriter, request *http.Request) (wasError bool) {
	switch e.(type) {
	case *StateForbiddenError:
		responseWriter.WriteHeader(http.StatusBadRequest)
		util.FprintDescriptionAndCodeAsJSON(responseWriter, 290008, "Cannot update an existing package.")
		return true
	case *NotFoundError:
		responseWriter.WriteHeader(http.StatusNotFound)
		util.FprintDescriptionAndCodeAsJSON(responseWriter, 10010, e.Error())
		return true
	case error:
		panic(e)
	}
	return false
}

func (handler *ResourceHandler) CopySourceGuid(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	if !HandleBodySizeLimits(responseWriter, request, handler.maxBodySizeLimit) {
		return
	}
	sourceGuid := sourceGuidFrom(request, responseWriter)
	if sourceGuid == "" {
		return // response is already handled in sourceGuidFrom
	}
	e := handler.blobstore.Copy(sourceGuid, params["identifier"])
	// TODO use Clock instead:
	writeResponseBasedOn("", e, responseWriter, request, http.StatusCreated, nil, &ResponseBody{Guid: params["identifier"], State: "READY", Type: "bits", CreatedAt: time.Now()}, "")
}

func sourceGuidFrom(request *http.Request, responseWriter http.ResponseWriter) string {
	content, e := ioutil.ReadAll(request.Body)
	util.PanicOnError(e)
	var payload struct {
		SourceGuid string `json:"source_guid"`
	}
	e = json.Unmarshal(content, &payload)
	if e != nil {
		badRequest(responseWriter, request, "Body must be valid JSON when request is not multipart/form-data. %+v", e)
		return ""
	}
	return payload.SourceGuid
}

func (handler *ResourceHandler) Head(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	exists, e := handler.blobstore.Exists(params["identifier"])
	util.PanicOnError(e)
	if exists {
		responseWriter.WriteHeader(http.StatusOK)
	} else {
		responseWriter.WriteHeader(http.StatusNotFound)
	}
}

func (handler *ResourceHandler) Get(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	var (
		redirectLocation string
		e                error
		body             io.ReadCloser
	)
	if handler.shouldProxyGetRequests {
		body, e = handler.blobstore.Get(params["identifier"])
	} else {
		body, redirectLocation, e = handler.blobstore.GetOrRedirect(params["identifier"])
	}
	writeResponseBasedOn(redirectLocation, e, responseWriter, request, http.StatusOK, body, nil, request.Header.Get("If-None-Modify"))
}

func (handler *ResourceHandler) BuildpackMetadata(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	body, e := handler.blobstore.Get(params["identifier"] + "-metadata")
	writeResponseBasedOn("", e, responseWriter, request, http.StatusOK, body, nil, request.Header.Get("If-None-Modify"))
}

func (handler *ResourceHandler) Delete(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	// TODO nothing should be S3 specific here
	// this check is needed, because S3 does not return a NotFound on a Delete request:
	exists, e := handler.blobstore.Exists(params["identifier"])
	util.PanicOnError(e)
	if !exists {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}
	e = handler.blobstore.Delete(params["identifier"])

	writeResponseBasedOn("", e, responseWriter, request, http.StatusNoContent, nil, nil, "")
}

func (handler *ResourceHandler) DeleteDir(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	e := handler.blobstore.DeleteDir(params["identifier"])

	switch e.(type) {
	case *NotFoundError:
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}
	writeResponseBasedOn("", e, responseWriter, request, http.StatusNoContent, nil, nil, "")
}

var emptyReader = ioutil.NopCloser(bytes.NewReader(nil))

// TODO: this function probably does too many things and should be refactored
func writeResponseBasedOn(redirectLocation string, e error, responseWriter http.ResponseWriter, request *http.Request, statusCode int, body io.ReadCloser, jsonBody *ResponseBody, ifNoneModify string) {
	switch e.(type) {
	case *NotFoundError:
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	case *NoSpaceLeftError:
		http.Error(responseWriter, util.DescriptionAndCodeAsJSON(500000, "Request Entity Too Large"), http.StatusInsufficientStorage)
		return
	case error:
		panic(e)
	}
	if redirectLocation != "" {
		redirect(responseWriter, redirectLocation)
		return
	}
	if body != nil {
		buffer, eTag := bufferAndEtagFrom(body)
		logger.From(request).Debugw("Cache check", "if-none-modify", ifNoneModify, "etag", eTag)
		responseWriter.Header().Set("ETag", eTag)
		if ifNoneModify == eTag {
			responseWriter.WriteHeader(http.StatusNotModified)
			return
		}
		responseWriter.WriteHeader(statusCode)
		io.Copy(responseWriter, buffer)
		return
	}
	if jsonBody != nil {
		respBody, marshallingErr := json.Marshal(jsonBody)
		util.PanicOnError(marshallingErr)
		responseWriter.WriteHeader(statusCode)
		responseWriter.Write(respBody)
		return
	}
	responseWriter.WriteHeader(statusCode)
}

func redirect(responseWriter http.ResponseWriter, redirectLocation string) {
	responseWriter.Header().Set("Location", redirectLocation)
	responseWriter.WriteHeader(http.StatusFound)
}

func badRequest(responseWriter http.ResponseWriter, request *http.Request, message string, args ...interface{}) {
	responseBody := fmt.Sprintf(message, args...)
	logger.From(request).Infow("Bad request", "body", responseBody)
	responseWriter.WriteHeader(http.StatusBadRequest)
	util.FprintDescriptionAndCodeAsJSON(responseWriter, 290003, message, args...)
}

func bufferAndEtagFrom(body io.ReadCloser) (buffer *bytes.Buffer, eTag string) {
	defer body.Close()
	var buf bytes.Buffer
	sha := sha1.New()
	_, e := io.Copy(io.MultiWriter(&buf, sha), body)
	util.PanicOnError(e)
	eTag = hex.EncodeToString(sha.Sum(nil))
	buffer = &buf
	return
}
