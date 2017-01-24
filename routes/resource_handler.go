package routes

import (
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
	handleBlobstoreResult(redirectLocation, e, responseWriter)
}

func (handler *ResourceHandler) copySourceGuid(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Body == nil {
		badRequest(responseWriter, "Body must contain source_guid when request is not multipart/form-data")
		return
	}
	sourceGuid := sourceGuidFrom(request.Body, responseWriter)
	if sourceGuid == "" {
		return // response is already handled in sourceGuidFrom
	}
	redirectLocation, e := handler.blobstore.Copy(sourceGuid, mux.Vars(request)["identifier"])
	if _, isNotFoundError := e.(*NotFoundError); isNotFoundError {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}
	handleBlobstoreResult(redirectLocation, e, responseWriter)
}

func sourceGuidFrom(body io.ReadCloser, responseWriter http.ResponseWriter) string {
	defer body.Close()
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

func handleBlobstoreResult(redirectLocation string, e error, responseWriter http.ResponseWriter) {
	if e != nil {
		internalServerError(responseWriter, e)
		return
	}

	if redirectLocation != "" {
		redirect(responseWriter, redirectLocation)
		return
	}

	responseWriter.WriteHeader(http.StatusCreated)
}

func (handler *ResourceHandler) Head(responseWriter http.ResponseWriter, request *http.Request) {
	redirectLocation, e := handler.blobstore.Head(mux.Vars(request)["identifier"])
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
	responseWriter.WriteHeader(http.StatusOK)
}

func (handler *ResourceHandler) Get(responseWriter http.ResponseWriter, request *http.Request) {
	body, redirectLocation, e := handler.blobstore.Get(mux.Vars(request)["identifier"])
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
	defer body.Close()
	responseWriter.WriteHeader(http.StatusOK)
	io.Copy(responseWriter, body)
}

func (handler *ResourceHandler) Delete(responseWriter http.ResponseWriter, request *http.Request) {
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
	if e != nil {
		internalServerError(responseWriter, e)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *ResourceHandler) DeleteDir(responseWriter http.ResponseWriter, request *http.Request) {
	e := handler.blobstore.DeletePrefix(mux.Vars(request)["identifier"])
	if e != nil {
		writeResponseBasedOnError(responseWriter, e)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func writeResponseBasedOnError(responseWriter http.ResponseWriter, e error) {
	switch e.(type) {
	case *NotFoundError:
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	case error:
		internalServerError(responseWriter, e)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
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
	responseWriter.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(responseWriter, message, args...)
}
