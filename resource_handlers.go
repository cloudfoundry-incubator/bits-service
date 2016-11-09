package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type BuildpackCacheHandler struct {
	blobStore BlobStore
}

func (handler *BuildpackCacheHandler) Put(responseWriter http.ResponseWriter, request *http.Request) {
	file, _, e := request.FormFile("buildpack_cache")
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(400)
		fmt.Fprintf(responseWriter, "Could not retrieve buildpack_cache form parameter")
		return
	}
	defer file.Close()
	handler.blobStore.Put(
		fmt.Sprintf("/buildpack_cache/entries/%s/%s", mux.Vars(request)["app_guid"], mux.Vars(request)["stack_name"]),
		file, responseWriter)
}

func (handler *BuildpackCacheHandler) Get(responseWriter http.ResponseWriter, request *http.Request) {
	handler.blobStore.Get(
		fmt.Sprintf("/buildpack_cache/entries/%s/%s", mux.Vars(request)["app_guid"], mux.Vars(request)["stack_name"]),
		responseWriter)
}

func (handler *BuildpackCacheHandler) Delete(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO
}

func (handler *BuildpackCacheHandler) DeleteAppGuid(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO
}

func (handler *BuildpackCacheHandler) DeleteEntries(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO
}

type ResourceHandler struct {
	blobStore    BlobStore
	resourceType string
}

func (handler *ResourceHandler) Put(responseWriter http.ResponseWriter, request *http.Request) {
	file, _, e := request.FormFile(handler.resourceType)
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(400)
		fmt.Fprintf(responseWriter, "Could not retrieve '%s' form parameter", handler.resourceType)
		return
	}
	defer file.Close()
	handler.blobStore.Put(pathFor(handler.resourceType, mux.Vars(request)["guid"]), file, responseWriter)
}

func (handler *ResourceHandler) Get(responseWriter http.ResponseWriter, request *http.Request) {
	handler.blobStore.Get(pathFor(handler.resourceType, mux.Vars(request)["guid"]), responseWriter)
}

func (handler *ResourceHandler) Delete(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO
}

func pathFor(resourceType string, identifier string) string {
	return fmt.Sprintf("/%s/%s/%s/%s", resourceType, identifier[0:2], identifier[2:4], identifier)
}
