package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"net/url"

	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo/local_blobstore"
	"github.com/petergtz/bitsgo/pathsigner"
	"github.com/petergtz/bitsgo/routes"
	"github.com/petergtz/bitsgo/s3_blobstore"
	"github.com/urfave/negroni"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	configPath = kingpin.Flag("config", "specify config to use").Required().Short('c').String()
)

func main() {
	kingpin.Parse()

	config, e := LoadConfig(*configPath)
	if e != nil {
		log.Fatalf("Could not load config. Caused by: %v", e)
	}

	rootRouter := mux.NewRouter()

	internalRouter := mux.NewRouter()

	privateEndpoint, e := url.Parse(config.PrivateEndpoint)
	if e != nil {
		log.Fatalf("Private endpoint invalid: %v", e)
	}
	rootRouter.Host(privateEndpoint.Host).Handler(internalRouter)

	publicEndpoint, e := url.Parse(config.PublicEndpoint)
	if e != nil {
		log.Fatalf("Public endpoint invalid: %v", e)
	}
	rootRouter.Host(privateEndpoint.Host).Handler(internalRouter)
	packageBlobstore, signPackageURLHandler := createPackageBlobstoreAndSignURLHandler(config.Packages, publicEndpoint.Host, config.Port, config.Secret)
	dropletBlobstore, signDropletURLHandler := createPackageBlobstoreAndSignURLHandler(config.Droplets, publicEndpoint.Host, config.Port, config.Secret)
	buildpackBlobstore, signBuildpackURLHandler := createPackageBlobstoreAndSignURLHandler(config.Buildpacks, publicEndpoint.Host, config.Port, config.Secret)

	routes.SetUpSignRoute(internalRouter, signPackageURLHandler, signDropletURLHandler, signBuildpackURLHandler)

	routes.SetUpPackageRoutes(internalRouter, packageBlobstore)
	routes.SetUpBuildpackRoutes(internalRouter, buildpackBlobstore)
	routes.SetUpDropletRoutes(internalRouter, dropletBlobstore)
	routes.SetUpBuildpackCacheRoutes(internalRouter, dropletBlobstore)

	if usesLocalBlobstore(config) {
		publicRouter := mux.NewRouter()
		rootRouter.Host(publicEndpoint.Host).Handler(negroni.New(
			&local_blobstore.SignatureVerificationMiddleware{&pathsigner.PathSigner{config.Secret}},
			negroni.Wrap(publicRouter),
		))
		if config.Packages.BlobstoreType == "local" {
			routes.SetUpPackageRoutes(publicRouter, packageBlobstore)
		}
		if config.Buildpacks.BlobstoreType == "local" {
			routes.SetUpBuildpackRoutes(publicRouter, buildpackBlobstore)
		}
		if config.Droplets.BlobstoreType == "local" {
			routes.SetUpDropletRoutes(publicRouter, dropletBlobstore)
			routes.SetUpBuildpackCacheRoutes(publicRouter, dropletBlobstore)
		}
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

func createPackageBlobstoreAndSignURLHandler(blobstoreConfig BlobstoreConfig, publicHost string, port int, secret string) (routes.Blobstore, routes.SignURLHandler) {
	switch blobstoreConfig.BlobstoreType {
	case "local":
		fmt.Println("Creating local blobstore", "path prefix:", blobstoreConfig.LocalConfig.PathPrefix)
		return local_blobstore.NewLocalBlobstore(blobstoreConfig.LocalConfig.PathPrefix),
			&local_blobstore.SignLocalUrlHandler{
				DelegateEndpoint: fmt.Sprintf("http://%v:%v", publicHost, port),
				Signer:           &pathsigner.PathSigner{secret},
			}
	case "s3":
		return s3_blobstore.NewS3LegacyBlobstore(
				blobstoreConfig.S3Config.Bucket,
				blobstoreConfig.S3Config.AccessKeyID,
				blobstoreConfig.S3Config.SecretAccessKey),
			s3_blobstore.NewSignS3UrlHandler(
				blobstoreConfig.S3Config.Bucket,
				blobstoreConfig.S3Config.AccessKeyID,
				blobstoreConfig.S3Config.SecretAccessKey)
	default:
		log.Fatalf("blobstoreConfig is invalid. BlobstoreType missing.")
		return nil, nil // satisfy compiler
	}
}

func usesLocalBlobstore(config Config) bool {
	return config.Packages.BlobstoreType == "local" ||
		config.Buildpacks.BlobstoreType == "local" ||
		config.Droplets.BlobstoreType == "local"
}
