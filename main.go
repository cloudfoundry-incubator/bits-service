package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"path/filepath"

	"net/url"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()
	packageHandler := &PackageHandler{blobStore: &LocalBlobStore{pathPrefix: "/tmp"}}
	signHandler := &SignHandler{}
	internalHostName := "internal.127.0.0.1.xip.io"
	publicHostName := "public.127.0.0.1.xip.io"

	internalRouter := router.Host(internalHostName).Subrouter()
	publicRouter := router.Host(publicHostName).Subrouter()

	signedUrlHandler := &SignedUrlHandler{
		internalRouter:   internalRouter,
		internalHostName: internalHostName,
	}

	internalRouter.Path("/packages/{guid}").Methods("PUT").HandlerFunc(packageHandler.put)
	internalRouter.Path("/packages/{guid}").Methods("GET").HandlerFunc(packageHandler.get)
	internalRouter.Path("/packages/{guid}").Methods("DELETE").HandlerFunc(packageHandler.delete)

	internalRouter.Path("/sign/{path}").Methods("GET").HandlerFunc(signHandler.Sign)

	publicRouter.PathPrefix("/signed").Methods("GET", "PUT").HandlerFunc(signedUrlHandler.decode)

	srv := &http.Server{
		Handler:      router,
		Addr:         "0.0.0.0:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

type BlobStore interface {
	Exists(path string) bool
	Get(path string) io.Reader
	Put(path string, src io.Reader)
	Delete(path string)
}

type LocalBlobStore struct {
	pathPrefix string
}

func (blobstore *LocalBlobStore) Exists(path string) bool {
	_, err := os.Stat(filepath.Join(blobstore.pathPrefix, path))
	return err == nil
}
func (blobstore *LocalBlobStore) Get(path string) io.Reader {
	file, e := os.Open(filepath.Join(blobstore.pathPrefix, path))
	if e != nil {
		panic(e)
	}
	return file
}

func (blobstore *LocalBlobStore) Put(path string, src io.Reader) {
	// TODO
}

func (blobstore *LocalBlobStore) Delete(path string) {
	// TODO
}

type PackageHandler struct {
	blobStore BlobStore
}

func (handler *PackageHandler) put(responseWriter http.ResponseWriter, request *http.Request) {
	file, _, e := request.FormFile("package")
	if e != nil {
		panic(e)
	}
	defer file.Close()
	handler.blobStore.Put("/packages/"+partitionedKey(mux.Vars(request)["guid"]), file)
}

func (handler *PackageHandler) get(responseWriter http.ResponseWriter, request *http.Request) {
	blob := handler.blobStore.Get("/packages/" + partitionedKey(mux.Vars(request)["guid"]))
	io.Copy(responseWriter, blob)
}

func partitionedKey(guid string) string {
	return filepath.Join(guid[0:2], guid[2:4], guid)
}

func (handler *PackageHandler) delete(responseWriter http.ResponseWriter, request *http.Request) {
	handler.blobStore.Delete("/packages/" + partitionedKey(mux.Vars(request)["guid"]))
}

type SignHandler struct {
}

func (handler *SignHandler) Sign(responseWriter http.ResponseWriter, request *http.Request) {

}

type SignedUrlHandler struct {
	internalRouter   *mux.Router
	internalHostName string
}

func (handler *SignedUrlHandler) decode(responseWriter http.ResponseWriter, request *http.Request) {
	if !validSignature(request) {
		responseWriter.WriteHeader(403)
		return
	}
	r := *request
	url, e := url.ParseRequestURI(strings.Replace(request.URL.String(), "/signed", "", 1))
	if e != nil {
		log.Fatal(e)
	}
	r.URL = url
	r.RequestURI = url.RequestURI()
	r.Host = handler.internalHostName
	spew.Dump(r)
	handler.internalRouter.ServeHTTP(responseWriter, &r)
}

func validSignature(request *http.Request) bool {
	return true
}
