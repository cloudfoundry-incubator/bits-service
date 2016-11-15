package main

import (
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
	config, e := LoadConfig("config.yml")
	if e != nil {
		log.Fatalf("Could load config. Caused by: %s", e)
	}

	rootRouter := mux.NewRouter()

	internalRouter := mux.NewRouter()
	rootRouter.Host(config.PrivateEndpoint).Handler(internalRouter)
	publicRouter := mux.NewRouter()
	rootRouter.Host(config.PublicEndpoint).Handler(negroni.New(
		&SignatureVerificationMiddleware{&PathSigner{config.Secret}},
		negroni.Wrap(publicRouter),
	))

	blobstore, signURLHandler := createBlobstoreAndSignURLHandler(config.PublicEndpoint, config.Port, config.Secret)

	SetUpSignRoute(internalRouter, signURLHandler)
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
		Addr:         fmt.Sprintf("0.0.0.0:%v", config.Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func createBlobstoreAndSignURLHandler(publicHostName string, port int, secret string) (Blobstore, SignedUrlHandler) {
	return &LocalBlobstore{pathPrefix: "/tmp"},
		&SignLocalUrlHandler{
			DelegateEndpoint: fmt.Sprintf("http://%v:%v", publicHostName, port),
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
