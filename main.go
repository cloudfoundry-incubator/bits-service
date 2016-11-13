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
		&SignatureVerificationMiddleware{&PathSigner{secret}},
		negroni.Wrap(publicRouter),
	))

	blobstore, signedURLHandler := createBlobstoreAndSignedURLHandler()

	setUpSignRoute(internalRouter, signedURLHandler)
	for _, router := range []*mux.Router{internalRouter, publicRouter} {
		SetUpPackageRoutes(router, blobstore)
		SetUpBuildpackRoutes(router, blobstore)
		SetUpDropletRoutes(router, blobstore)
		SetUpBuildpackCacheRoutes(router, blobstore)
	}

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

func createBlobstoreAndSignedURLHandler() (Blobstore, SignedUrlHandler) {
	return &LocalBlobstore{pathPrefix: "/tmp"},
		&SignedLocalUrlHandler{
			DelegateEndpoint: "http://" + publicHostName + ":" + port,
			Signer:           &PathSigner{secret},
		}
}

func setUpSignRoute(router *mux.Router, signedUrlHandler SignedUrlHandler) {
	router.PathPrefix("/sign/").Methods("GET").HandlerFunc(signedUrlHandler.Sign)
}

func SetUpPackageRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &ResourceHandler{blobstore: blobstore, resourceType: "package"}
	router.Path("/packages/{guid}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/packages/{guid}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/packages/{guid}").Methods("DELETE").HandlerFunc(handler.Delete)
}

func SetUpBuildpackRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &ResourceHandler{blobstore: blobstore, resourceType: "buildpack"}
	router.Path("/buildpacks/{guid}").Methods("PUT").HandlerFunc(handler.Put)
	// TODO change Put/Get/etc. signature to allow this:
	// router.Path("/buildpacks/{guid}").Methods("PUT").HandlerFunc(delegateTo(handler.Put))
	router.Path("/buildpacks/{guid}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/buildpacks/{guid}").Methods("DELETE").HandlerFunc(handler.Delete)
}

func delegateTo(delegate func(http.ResponseWriter, *http.Request, map[string]string)) func(http.ResponseWriter, *http.Request) {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		delegate(responseWriter, request, mux.Vars(request))
	}
}

func SetUpDropletRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &ResourceHandler{blobstore: blobstore, resourceType: "droplet"}
	router.Path("/droplets/{guid}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/droplets/{guid}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/droplets/{guid}").Methods("DELETE").HandlerFunc(handler.Delete)
}

func SetUpBuildpackCacheRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &BuildpackCacheHandler{blobStore: blobstore}
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("DELETE").HandlerFunc(handler.Delete)
	router.Path("/buildpack_cache/entries/{app_guid}/").Methods("DELETE").HandlerFunc(handler.DeleteAppGuid)
	router.Path("/buildpack_cache/entries").Methods("DELETE").HandlerFunc(handler.DeleteEntries)
}

type Blobstore interface {
	Get(path string, responseWriter http.ResponseWriter)
	Put(path string, src io.ReadSeeker, responseWriter http.ResponseWriter)
}

type SignedUrlHandler interface {
	Sign(responseWriter http.ResponseWriter, request *http.Request)
}
