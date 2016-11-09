package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

const (
	internalHostName = "internal.127.0.0.1.xip.io"
	publicHostName   = "public.127.0.0.1.xip.io"
	port             = "8000"
	secret           = "geheim"
)

func main() {
	router := mux.NewRouter()

	internalRouter := mux.NewRouter()
	publicRouter := mux.NewRouter()
	router.Host(internalHostName).Handler(internalRouter)
	router.Host(publicHostName).Handler(negroni.New(
		&SignatureVerifier{Secret: secret},
		negroni.Wrap(publicRouter),
	))

	blobstore, signedURLHandler := createBlobstoreAndSignedURLHandler()

	setUpSignRoute(internalRouter, signedURLHandler)
	setUpPackageRoutes(internalRouter, blobstore)
	setUpPackageRoutes(publicRouter, blobstore)
	setUpBuildpackRoutes(internalRouter, blobstore)
	setUpBuildpackRoutes(publicRouter, blobstore)
	setUpDropletRoutes(internalRouter, blobstore)
	setUpDropletRoutes(publicRouter, blobstore)
	setUpBuildpackCacheRoutes(internalRouter, blobstore)
	setUpBuildpackCacheRoutes(publicRouter, blobstore)

	srv := &http.Server{
		Handler: negroni.New(
			&negroni.Logger{log.New(os.Stdout, "[bitsgo] ", log.LstdFlags|log.Lshortfile|log.LUTC)},
			negroni.Wrap(router),
		),
		Addr:         "0.0.0.0:" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func createBlobstoreAndSignedURLHandler() (BlobStore, SignedUrlHandler) {
	return &LocalBlobStore{pathPrefix: "/tmp"},
		&SignedLocalUrlHandler{
			DelegateEndpoint: "http://" + publicHostName + ":" + port,
			Secret:           secret,
		}
}

func setUpSignRoute(router *mux.Router, signedUrlHandler SignedUrlHandler) {
	router.PathPrefix("/sign/").Methods("GET").HandlerFunc(signedUrlHandler.Sign)
}

func setUpPackageRoutes(router *mux.Router, blobStore BlobStore) {
	handler := &ResourceHandler{blobStore: blobStore, resourceType: "package"}
	router.Path("/packages/{guid}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/packages/{guid}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/packages/{guid}").Methods("DELETE").HandlerFunc(handler.Delete)
}

func setUpBuildpackRoutes(router *mux.Router, blobStore BlobStore) {
	handler := &ResourceHandler{blobStore: blobStore, resourceType: "buildpack"}
	router.Path("/buildpacks/{guid}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/buildpacks/{guid}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/buildpacks/{guid}").Methods("DELETE").HandlerFunc(handler.Delete)
}

func setUpDropletRoutes(router *mux.Router, blobStore BlobStore) {
	handler := &ResourceHandler{blobStore: blobStore, resourceType: "droplet"}
	router.Path("/droplets/{guid}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/droplets/{guid}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/droplets/{guid}").Methods("DELETE").HandlerFunc(handler.Delete)
}

func setUpBuildpackCacheRoutes(router *mux.Router, blobStore BlobStore) {
	handler := &BuildpackCacheHandler{blobStore: blobStore}
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("DELETE").HandlerFunc(handler.Delete)
	router.Path("/buildpack_cache/entries/{app_guid}/").Methods("DELETE").HandlerFunc(handler.DeleteAppGuid)
	router.Path("/buildpack_cache/entries").Methods("DELETE").HandlerFunc(handler.DeleteEntries)
}

type BlobStore interface {
	Get(path string, responseWriter http.ResponseWriter)
	Put(path string, src io.ReadSeeker, responseWriter http.ResponseWriter)
}

type SignedUrlHandler interface {
	Sign(responseWriter http.ResponseWriter, request *http.Request)
}
