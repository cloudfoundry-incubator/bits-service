package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

func main() {
	router := mux.NewRouter()
	internalHostName := "internal.127.0.0.1.xip.io"
	publicHostName := "public.127.0.0.1.xip.io"

	internalRouter := mux.NewRouter()
	publicRouter := mux.NewRouter()
	router.Host(internalHostName).Handler(internalRouter)
	router.Host(publicHostName).Handler(negroni.New(
		&SignatureVerifier{Secret: "geheim"},
		negroni.Wrap(publicRouter),
	))

	blobstore := &LocalBlobStore{pathPrefix: "/tmp"}
	signedURLHandler := &SignedUrlHandler{
		DelegateEndpoint: "http://" + publicHostName + ":8000",
		Secret:           "geheim",
	}

	setUpPackageRoutes(internalRouter, blobstore)
	setUpPackageRoutes(publicRouter, blobstore)
	internalRouter.PathPrefix("/sign/").Methods("GET").HandlerFunc(signedURLHandler.Sign)

	srv := &http.Server{
		Handler: negroni.New(
			&negroni.Logger{log.New(os.Stdout, "[bitsgo] ", log.LstdFlags|log.Lshortfile|log.LUTC)},
			negroni.Wrap(router),
		),
		Addr:         "0.0.0.0:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

type SignatureVerifier struct {
	Secret string
}

func (l *SignatureVerifier) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if r.URL.Query().Get("md5") == "" {
		rw.WriteHeader(403)
		return
	}
	if r.URL.Query().Get("md5") != fmt.Sprintf("%x", md5.Sum([]byte(r.URL.Path+l.Secret))) {
		rw.WriteHeader(403)
		return
	}

	next(rw, r)
}

func setUpPackageRoutes(router *mux.Router, blobStore BlobStore) {
	packageHandler := &ResourceHandler{blobStore: blobStore, resourceType: "package"}
	router.Path("/packages/{guid}").Methods("PUT").HandlerFunc(packageHandler.Put)
	router.Path("/packages/{guid}").Methods("GET").HandlerFunc(packageHandler.Get)
	router.Path("/packages/{guid}").Methods("DELETE").HandlerFunc(packageHandler.Delete)
}

type BlobStore interface {
	Get(path string, responseWriter http.ResponseWriter)
	Put(path string, src io.ReadSeeker, responseWriter http.ResponseWriter)
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
