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
	rootRouter := mux.NewRouter()

	internalRouter := mux.NewRouter()
	rootRouter.Host(internalHostName).Handler(internalRouter)
	publicRouter := mux.NewRouter()
	rootRouter.Host(publicHostName).Handler(negroni.New(
		&SignatureVerificationMiddleware{&PathSigner{secret}},
		negroni.Wrap(publicRouter),
	))

	blobstore, signedURLHandler := createBlobstoreAndSignedURLHandler()

	SetUpSignRoute(internalRouter, signedURLHandler)
	for _, router := range []*mux.Router{internalRouter, publicRouter} {
		SetUpPackageRoutes(router, blobstore)
		SetUpBuildpackRoutes(router, blobstore)
		SetUpDropletRoutes(router, blobstore)
		SetUpBuildpackCacheRoutes(router, blobstore)
	}

	srv := &http.Server{
		Handler: negroni.New(
			&negroni.Logger{log.New(os.Stdout, "[bitsgo] ", log.LstdFlags|log.Lshortfile|log.LUTC)},
			negroni.Wrap(rootRouter),
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

func SetUpSignRoute(router *mux.Router, signedUrlHandler SignedUrlHandler) {
	router.PathPrefix("/sign/").Methods("GET").HandlerFunc(signedUrlHandler.Sign)
}

type Blobstore interface {
	Get(path string, responseWriter http.ResponseWriter)
	Put(path string, src io.ReadSeeker, responseWriter http.ResponseWriter)
}

type SignedUrlHandler interface {
	Sign(responseWriter http.ResponseWriter, request *http.Request)
}
