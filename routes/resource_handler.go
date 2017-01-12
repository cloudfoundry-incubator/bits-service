package routes

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

type ResourceHandler struct {
	blobstore    Blobstore
	resourceType string
}

func (handler *ResourceHandler) Put(responseWriter http.ResponseWriter, request *http.Request) {
	file, _, e := request.FormFile(handler.resourceType)
	if e != nil {
		badRequest(responseWriter, "Could not retrieve '%s' form parameter", handler.resourceType)
		return
	}
	defer file.Close()

	redirectLocation, e := handler.blobstore.Put(mux.Vars(request)["identifier"], file)

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
	panic("DeleteDir nt implemented yet")
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
	log.Printf("Internal Server Error. Caused by: %+v", errors.WithStack(e))
	responseWriter.WriteHeader(http.StatusInternalServerError)
}

func badRequest(responseWriter http.ResponseWriter, message string, args ...interface{}) {
	responseWriter.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(responseWriter, message, args...)
}
