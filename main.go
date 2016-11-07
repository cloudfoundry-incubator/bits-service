package main

import (
	"io"
	"log"
	"net/http"
	"time"

	"path/filepath"

	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()
	packageHandler := &PackageHandler{blobStore: &LocalBlobStore{pathPrefix: "/tmp"}}
	internalHostName := "internal.127.0.0.1.xip.io"
	publicHostName := "public.127.0.0.1.xip.io"

	internalRouter := mux.NewRouter()
	publicRouter := mux.NewRouter()
	router.Host(internalHostName).Handler(internalRouter)
	router.Host(publicHostName).Handler(publicRouter)

	internalRouter.Path("/packages/{guid}").Methods("PUT").HandlerFunc(packageHandler.Put)
	internalRouter.Path("/packages/{guid}").Methods("GET").HandlerFunc(packageHandler.Get)
	internalRouter.Path("/packages/{guid}").Methods("DELETE").HandlerFunc(packageHandler.Delete)

	signedURLHandler := &SignedUrlHandler{
		Delegate:         internalRouter,
		DelegateEndpoint: "http://" + publicHostName + ":8000",
		Secret:           "geheim",
	}
	internalRouter.PathPrefix("/sign/").Methods("GET").HandlerFunc(signedURLHandler.Sign)
	publicRouter.PathPrefix("/signed/").HandlerFunc(signedURLHandler.Decode)

	srv := &http.Server{
		Handler:      router,
		Addr:         "0.0.0.0:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

type BlobStore interface {
	Get(path string, responseWriter http.ResponseWriter)
	Put(path string, src io.ReadSeeker, responseWriter http.ResponseWriter)
}

type PackageHandler struct {
	blobStore BlobStore
}

func (handler *PackageHandler) Put(responseWriter http.ResponseWriter, request *http.Request) {
	file, _, e := request.FormFile("package")
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(400)
		responseWriter.Write([]byte("Could not retrieve 'package' form parameter"))
		return
	}
	defer file.Close()
	handler.blobStore.Put("/packages/"+partitionedKey(mux.Vars(request)["guid"]), file, responseWriter)
}

func (handler *PackageHandler) Get(responseWriter http.ResponseWriter, request *http.Request) {
	handler.blobStore.Get("/packages/"+partitionedKey(mux.Vars(request)["guid"]), responseWriter)
}

func (handler *PackageHandler) Delete(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO
}

func partitionedKey(guid string) string {
	return filepath.Join(guid[0:2], guid[2:4], guid)
}
