package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/uber-go/zap"
)

type ResourceHandler struct {
	blobstore    Blobstore
	resourceType string
}

var (
	logger = zap.New(zap.NewTextEncoder(), zap.DebugLevel, zap.AddCaller())
)

func (handler *ResourceHandler) Put(responseWriter http.ResponseWriter, request *http.Request) {
	var (
		redirectLocation string
		e                error
	)
	if strings.Contains(request.Header.Get("Content-Type"), "multipart/form-data") {
		logger.Info("Multipart upload")
		file, _, e := request.FormFile(handler.resourceType)
		if e != nil {
			badRequest(responseWriter, "Could not retrieve '%s' form parameter", handler.resourceType)
			return
		}
		defer file.Close()

		redirectLocation, e = handler.blobstore.Put(mux.Vars(request)["identifier"], file)
	} else {
		logger.Info("Copy source guid")
		redirectLocation, e = handler.copySourceGuid(request.Body, mux.Vars(request)["identifier"], responseWriter)
	}

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

	responseWriter.WriteHeader(http.StatusCreated)
}

func (handler *ResourceHandler) copySourceGuid(body io.ReadCloser, targetGuid string, responseWriter http.ResponseWriter) (string, error) {
	if body == nil {
		badRequest(responseWriter, "Body must contain source_guid when request is not multipart/form-data")
		return "", nil
	}
	defer body.Close()
	content, e := ioutil.ReadAll(body)
	if e != nil {
		internalServerError(responseWriter, e)
		return "", nil
	}
	var payload struct {
		SourceGuid string `json:"source_guid"`
	}
	e = json.Unmarshal(content, &payload)
	if e != nil {
		badRequest(responseWriter, "Body must be valid JSON when request is not multipart/form-data. %+v", e)
		return "", nil
	}
	return handler.blobstore.Copy(payload.SourceGuid, targetGuid)
}

func (handler *ResourceHandler) Head(responseWriter http.ResponseWriter, request *http.Request) {
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
	// TODO this should actually be logged as part of the middleware, so that it is easier to map it to a specific request
	log.Printf("Location: %v", redirectLocation)
	responseWriter.Header().Set("Location", redirectLocation)
	responseWriter.WriteHeader(http.StatusFound)
}

func internalServerError(responseWriter http.ResponseWriter, e error) {
	log.Printf("Internal Server Error. Caused by: %+v", e)
	responseWriter.WriteHeader(http.StatusInternalServerError)
}

func badRequest(responseWriter http.ResponseWriter, message string, args ...interface{}) {
	responseWriter.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(responseWriter, message, args...)
}
