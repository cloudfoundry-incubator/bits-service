package routes

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
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

	redirectLocation, e := handler.blobstore.Put(pathFor(mux.Vars(request)["guid"]), file)

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
	body, redirectLocation, e := handler.blobstore.Get(pathFor(mux.Vars(request)["guid"]))
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
	body, redirectLocation, e := handler.blobstore.Get(pathFor(mux.Vars(request)["guid"]))
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
	exists, e := handler.blobstore.Exists(pathFor(mux.Vars(request)["guid"]))
	if e != nil {
		internalServerError(responseWriter, e)
		return
	}
	if !exists {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}

	e = handler.blobstore.Delete(pathFor(mux.Vars(request)["guid"]))
	if e != nil {
		internalServerError(responseWriter, e)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func pathFor(identifier string) string {
	return fmt.Sprintf("/%s/%s/%s", identifier[0:2], identifier[2:4], identifier)
}

type BuildpackCacheHandler struct {
	blobStore Blobstore
}

func (handler *BuildpackCacheHandler) Put(responseWriter http.ResponseWriter, request *http.Request) {
	file, _, e := request.FormFile("buildpack_cache")
	if e != nil {
		badRequest(responseWriter, "Could not retrieve buildpack_cache form parameter")
		return
	}
	defer file.Close()

	redirectLocation, e := handler.blobStore.Put(
		fmt.Sprintf("/buildpack_cache/%s/%s", pathFor(mux.Vars(request)["app_guid"]), mux.Vars(request)["stack_name"]), file)

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

func (handler *BuildpackCacheHandler) Head(responseWriter http.ResponseWriter, request *http.Request) {
	body, redirectLocation, e := handler.blobStore.Get(
		fmt.Sprintf("/buildpack_cache/%s/%s", pathFor(mux.Vars(request)["app_guid"]), mux.Vars(request)["stack_name"]))
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

func (handler *BuildpackCacheHandler) Get(responseWriter http.ResponseWriter, request *http.Request) {
	body, redirectLocation, e := handler.blobStore.Get(
		fmt.Sprintf("/buildpack_cache/%s/%s", pathFor(mux.Vars(request)["app_guid"]), mux.Vars(request)["stack_name"]))
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

func (handler *BuildpackCacheHandler) Delete(responseWriter http.ResponseWriter, request *http.Request) {
	e := handler.blobStore.Delete(
		fmt.Sprintf("/buildpack_cache/%s/%s", pathFor(mux.Vars(request)["app_guid"]), mux.Vars(request)["stack_name"]))
	switch e.(type) {
	case *NotFoundError:
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	case error:
		internalServerError(responseWriter, e)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BuildpackCacheHandler) DeleteAppGuid(responseWriter http.ResponseWriter, request *http.Request) {
	e := handler.blobStore.Delete(
		fmt.Sprintf("/buildpack_cache/%s", pathFor(mux.Vars(request)["app_guid"])))
	writeResponseBasedOnError(responseWriter, e)
}

func (handler *BuildpackCacheHandler) DeleteEntries(responseWriter http.ResponseWriter, request *http.Request) {
	e := handler.blobStore.Delete("/buildpack_cache")
	writeResponseBasedOnError(responseWriter, e)
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
