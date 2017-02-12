package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo/logger"
	"github.com/uber-go/zap"
)

type ResourceHandler struct {
	blobstore    Blobstore
	resourceType string
}

func (handler *ResourceHandler) Put(responseWriter http.ResponseWriter, request *http.Request) {
	if strings.Contains(request.Header.Get("Content-Type"), "multipart/form-data") {
		logger.From(request).Debug("Multipart upload")
		handler.uploadMultipart(responseWriter, request)
	} else {
		logger.From(request).Debug("Copy source guid")
		handler.copySourceGuid(responseWriter, request)
	}
}

func (handler *ResourceHandler) uploadMultipart(responseWriter http.ResponseWriter, request *http.Request) {
	file, _, e := request.FormFile(handler.resourceType)
	if e != nil {
		badRequest(responseWriter, "Could not retrieve '%s' form parameter", handler.resourceType)
		return
	}
	defer file.Close()

	redirectLocation, e := handler.blobstore.Put(mux.Vars(request)["identifier"], file)
	writeResponseBasedOn(redirectLocation, e, responseWriter, http.StatusCreated, emptyReader)
}

func (handler *ResourceHandler) copySourceGuid(responseWriter http.ResponseWriter, request *http.Request) {
	sourceGuid := sourceGuidFrom(request.Body, responseWriter)
	if sourceGuid == "" {
		return // response is already handled in sourceGuidFrom
	}
	redirectLocation, e := handler.blobstore.Copy(sourceGuid, mux.Vars(request)["identifier"])
	writeResponseBasedOn(redirectLocation, e, responseWriter, http.StatusCreated, emptyReader)
}

func sourceGuidFrom(body io.ReadCloser, responseWriter http.ResponseWriter) string {
	content, e := ioutil.ReadAll(body)
	if e != nil {
		internalServerError(responseWriter, e)
		return ""
	}
	var payload struct {
		SourceGuid string `json:"source_guid"`
	}
	e = json.Unmarshal(content, &payload)
	if e != nil {
		badRequest(responseWriter, "Body must be valid JSON when request is not multipart/form-data. %+v", e)
		return ""
	}
	return payload.SourceGuid
}

func (handler *ResourceHandler) Head(responseWriter http.ResponseWriter, request *http.Request) {
	redirectLocation, e := handler.blobstore.Head(mux.Vars(request)["identifier"])
	writeResponseBasedOn(redirectLocation, e, responseWriter, http.StatusOK, emptyReader)
}

func (handler *ResourceHandler) Get(responseWriter http.ResponseWriter, request *http.Request) {
	body, redirectLocation, e := handler.blobstore.Get(mux.Vars(request)["identifier"])
	writeResponseBasedOn(redirectLocation, e, responseWriter, http.StatusOK, body)
}

func (handler *ResourceHandler) Delete(responseWriter http.ResponseWriter, request *http.Request) {
	// this check is needed, because S3 does not return a NotFound on a Delete request:
	exists, e := handler.blobstore.Exists(mux.Vars(request)["identifier"])
	if e != nil {
		internalServerError(responseWriter, e)
		return
	}
	if !exists {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}
	e = handler.blobstore.Delete(mux.Vars(request)["identifier"])
	writeResponseBasedOn("", e, responseWriter, http.StatusNoContent, emptyReader)
}

func (handler *ResourceHandler) DeleteDir(responseWriter http.ResponseWriter, request *http.Request) {
	e := handler.blobstore.DeletePrefix(mux.Vars(request)["identifier"])
	switch e.(type) {
	case *NotFoundError:
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}
	writeResponseBasedOn("", e, responseWriter, http.StatusNoContent, emptyReader)
}

var emptyReader = ioutil.NopCloser(bytes.NewReader(nil))

func writeResponseBasedOn(redirectLocation string, e error, responseWriter http.ResponseWriter, statusCode int, responseReader io.ReadCloser) {
	switch e.(type) {
	case *NotFoundError:
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	case error:
		internalServerError(responseWriter, e)
		return
	}
	if redirectLocation != "" {
		redirect(responseWriter, redirectLocation)
		return
	}
	defer responseReader.Close()
	responseWriter.WriteHeader(statusCode)
	io.Copy(responseWriter, responseReader)
}

func redirect(responseWriter http.ResponseWriter, redirectLocation string) {
	responseWriter.Header().Set("Location", redirectLocation)
	responseWriter.WriteHeader(http.StatusFound)
}

func internalServerError(responseWriter http.ResponseWriter, e error) {
	logger.Log.Error("Internal Server Error.", zap.String("error", fmt.Sprintf("%+v", e)))
	responseWriter.WriteHeader(http.StatusInternalServerError)
}

func badRequest(responseWriter http.ResponseWriter, message string, args ...interface{}) {
	responseBody := fmt.Sprintf(message, args...)
	logger.Log.Debug("Bad rquest", zap.String("body", responseBody))
	responseWriter.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(responseWriter, responseBody)
}
