package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"net/url"

	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo/basic_auth_middleware"
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
	rootRouter.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

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
	appStashBlobstore := createAppStashBlobstore(config.AppStash)
	packageBlobstore, signPackageURLHandler := createBlobstoreAndSignURLHandler(config.Packages, publicEndpoint.Host, config.Port, config.Secret, "packages")
	dropletBlobstore, signDropletURLHandler := createBlobstoreAndSignURLHandler(config.Droplets, publicEndpoint.Host, config.Port, config.Secret, "droplets")
	buildpackBlobstore, signBuildpackURLHandler := createBlobstoreAndSignURLHandler(config.Buildpacks, publicEndpoint.Host, config.Port, config.Secret, "buildpacks")
	signBuildpackCacheURLHandler := createBuildpackCacheSignURLHandler(config.Droplets, publicEndpoint.Host, config.Port, config.Secret, "droplets")

	routes.SetUpSignRoute(internalRouter, &basic_auth_middleware.BasicAuthMiddleware{config.SigningUsers[0].Username, config.SigningUsers[0].Password},
		signPackageURLHandler, signDropletURLHandler, signBuildpackURLHandler, signBuildpackCacheURLHandler)

	routes.SetUpAppStashRoutes(internalRouter, appStashBlobstore)
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
			newLogger(),
			negroni.Wrap(rootRouter),
		),
		Addr: fmt.Sprintf("0.0.0.0:%v", config.Port),
		// TODO possibly remove timeouts completely?
		WriteTimeout: 5 * time.Minute,
		ReadTimeout:  5 * time.Minute,
	}

	log.Fatal(srv.ListenAndServe())
}

func createBlobstoreAndSignURLHandler(blobstoreConfig BlobstoreConfig, publicHost string, port int, secret string, resourceType string) (routes.Blobstore, *routes.SignResourceHandler) {
	switch blobstoreConfig.BlobstoreType {
	case "local", "LOCAL":
		fmt.Println("Creating local blobstore", "path prefix:", blobstoreConfig.LocalConfig.PathPrefix)
		return local_blobstore.NewLocalBlobstore(blobstoreConfig.LocalConfig.PathPrefix),
			routes.NewSignResourceHandler(
				&local_blobstore.LocalResourceSigner{
					DelegateEndpoint:   fmt.Sprintf("http://%v:%v", publicHost, port),
					Signer:             &pathsigner.PathSigner{secret},
					ResourcePathPrefix: "/" + resourceType + "/",
				},
			)
	case "s3", "S3", "AWS", "aws":
		return s3_blobstore.NewS3LegacyBlobstore(
				blobstoreConfig.S3Config.Bucket,
				blobstoreConfig.S3Config.AccessKeyID,
				blobstoreConfig.S3Config.SecretAccessKey,
				blobstoreConfig.S3Config.Region),
			routes.NewSignResourceHandler(
				s3_blobstore.NewS3ResourceSigner(
					blobstoreConfig.S3Config.Bucket,
					blobstoreConfig.S3Config.AccessKeyID,
					blobstoreConfig.S3Config.SecretAccessKey,
					blobstoreConfig.S3Config.Region),
			)
	default:
		log.Fatalf("blobstoreConfig is invalid. BlobstoreType missing.")
		return nil, nil // satisfy compiler
	}
}

func createBuildpackCacheSignURLHandler(blobstoreConfig BlobstoreConfig, publicHost string, port int, secret string, resourceType string) *routes.SignResourceHandler {
	switch blobstoreConfig.BlobstoreType {
	case "local", "LOCAL":
		fmt.Println("Creating local blobstore", "path prefix:", blobstoreConfig.LocalConfig.PathPrefix)
		return routes.NewSignResourceHandler(
			&local_blobstore.LocalResourceSigner{
				DelegateEndpoint:   fmt.Sprintf("http://%v:%v", publicHost, port),
				Signer:             &pathsigner.PathSigner{secret},
				ResourcePathPrefix: "/" + resourceType + "/",
			},
		)
	case "s3", "S3", "AWS", "aws":
		return routes.NewSignResourceHandler(
			s3_blobstore.NewS3BuildpackCacheSigner(
				blobstoreConfig.S3Config.Bucket,
				blobstoreConfig.S3Config.AccessKeyID,
				blobstoreConfig.S3Config.SecretAccessKey,
				blobstoreConfig.S3Config.Region),
		)
	default:
		log.Fatalf("blobstoreConfig is invalid. BlobstoreType missing.")
		return nil // satisfy compiler
	}
}

func createAppStashBlobstore(blobstoreConfig BlobstoreConfig) routes.Blobstore {
	switch blobstoreConfig.BlobstoreType {
	case "local", "LOCAL":
		fmt.Println("Creating local blobstore", "path prefix:", blobstoreConfig.LocalConfig.PathPrefix)
		return local_blobstore.NewLocalBlobstore(blobstoreConfig.LocalConfig.PathPrefix)

	case "s3", "S3", "AWS", "aws":
		return s3_blobstore.NewS3NoRedirectBlobStore(
			blobstoreConfig.S3Config.Bucket,
			blobstoreConfig.S3Config.AccessKeyID,
			blobstoreConfig.S3Config.SecretAccessKey,
			blobstoreConfig.S3Config.Region)
	default:
		log.Fatalf("blobstoreConfig is invalid. BlobstoreType missing.")
		return nil // satisfy compiler
	}
}

func usesLocalBlobstore(config Config) bool {
	return config.Packages.BlobstoreType == "local" ||
		config.Buildpacks.BlobstoreType == "local" ||
		config.Droplets.BlobstoreType == "local"
}

func newLogger() *negroni.Logger {
	logger := &negroni.Logger{ALogger: log.New(os.Stdout, "[bitsgo] ", log.LstdFlags|log.Lshortfile|log.LUTC)}
	logger.SetFormat(negroni.LoggerDefaultFormat)
	logger.SetDateFormat(negroni.LoggerDefaultDateFormat)
	return logger
}
