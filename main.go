package main

import (
	"fmt"
	"net/http"
	"time"

	"net/url"

	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo/basic_auth_middleware"
	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/local_blobstore"
	log "github.com/petergtz/bitsgo/logger"
	"github.com/petergtz/bitsgo/middlewares"
	"github.com/petergtz/bitsgo/pathsigner"
	"github.com/petergtz/bitsgo/routes"
	"github.com/petergtz/bitsgo/s3_blobstore"
	"github.com/uber-go/zap"
	"github.com/urfave/negroni"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	configPath = kingpin.Flag("config", "specify config to use").Required().Short('c').String()
)

func main() {
	kingpin.Parse()

	config, e := config.LoadConfig(*configPath)
	if e != nil {
		log.Log.Fatal("Could not load config.", zap.Error(e))
	}

	rootRouter := mux.NewRouter()
	rootRouter.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	internalRouter := mux.NewRouter()

	privateEndpoint, e := url.Parse(config.PrivateEndpoint)
	if e != nil {
		log.Log.Fatal("Private endpoint invalid", zap.Error(e))
	}
	rootRouter.Host(privateEndpoint.Host).Handler(internalRouter)

	publicEndpoint, e := url.Parse(config.PublicEndpoint)
	if e != nil {
		log.Log.Fatal("Public endpoint invalid", zap.Error(e))
	}
	appStashBlobstore := createAppStashBlobstore(config.AppStash)
	packageBlobstore, signPackageURLHandler := createBlobstoreAndSignURLHandler(config.Packages, publicEndpoint.Host, config.Port, config.Secret, "packages")
	dropletBlobstore, signDropletURLHandler := createBlobstoreAndSignURLHandler(config.Droplets, publicEndpoint.Host, config.Port, config.Secret, "droplets")
	buildpackBlobstore, signBuildpackURLHandler := createBlobstoreAndSignURLHandler(config.Buildpacks, publicEndpoint.Host, config.Port, config.Secret, "buildpacks")
	buildpackCacheBlobstore, signBuildpackCacheURLHandler := createBuildpackCacheSignURLHandler(config.Droplets, publicEndpoint.Host, config.Port, config.Secret, "droplets")

	routes.SetUpSignRoute(internalRouter, &basic_auth_middleware.BasicAuthMiddleware{config.SigningUsers[0].Username, config.SigningUsers[0].Password},
		signPackageURLHandler, signDropletURLHandler, signBuildpackURLHandler, signBuildpackCacheURLHandler)

	routes.SetUpAppStashRoutes(internalRouter, appStashBlobstore)
	routes.SetUpPackageRoutes(internalRouter, packageBlobstore)
	routes.SetUpBuildpackRoutes(internalRouter, buildpackBlobstore)
	routes.SetUpDropletRoutes(internalRouter, dropletBlobstore)
	routes.SetUpBuildpackCacheRoutes(internalRouter, buildpackCacheBlobstore)

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
		routes.SetUpBuildpackCacheRoutes(publicRouter, buildpackCacheBlobstore)
	}

	httpHandler := negroni.New(middlewares.NewZapLoggerMiddleware(log.Log))
	if config.MaxBodySizeBytes() != 0 {
		httpHandler.Use(middlewares.NewBodySizeLimitMiddleware(config.MaxBodySizeBytes()))
	}
	httpHandler.UseHandler(rootRouter)

	httpServer := &http.Server{
		Handler: httpHandler,
		Addr:    fmt.Sprintf("0.0.0.0:%v", config.Port),
		// TODO possibly remove timeouts completely?
		WriteTimeout: 5 * time.Minute,
		ReadTimeout:  5 * time.Minute,
	}

	e = httpServer.ListenAndServe()
	log.Log.Fatal("http server crashed", zap.Error(e))
}

func createBlobstoreAndSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicHost string, port int, secret string, resourceType string) (routes.Blobstore, *routes.SignResourceHandler) {
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
		return routes.DecorateWithPartitioningPathBlobstore(
				s3_blobstore.NewS3LegacyBlobstore(*blobstoreConfig.S3Config)),
			routes.NewSignResourceHandler(
				routes.DecorateWithPartitioningPathResourceSigner(
					s3_blobstore.NewS3ResourceSigner(*blobstoreConfig.S3Config)))
	default:
		log.Log.Fatal("blobstoreConfig is invalid. BlobstoreType missing.")
		return nil, nil // satisfy compiler
	}
}

func createBuildpackCacheSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicHost string, port int, secret string, resourceType string) (routes.Blobstore, *routes.SignResourceHandler) {
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
		return routes.DecorateWithPartitioningPathBlobstore(
				routes.DecorateWithPrefixingPathBlobstore(
					s3_blobstore.NewS3LegacyBlobstore(*blobstoreConfig.S3Config), "buildpack_cache/")),
			routes.NewSignResourceHandler(
				routes.DecorateWithPartitioningPathResourceSigner(
					routes.DecorateWithPrefixingPathResourceSigner(
						s3_blobstore.NewS3ResourceSigner(*blobstoreConfig.S3Config),
						"buildpack_cache")),
			)
	default:
		log.Log.Fatal("blobstoreConfig is invalid. BlobstoreType missing.")
		return nil, nil // satisfy compiler
	}
}

func createAppStashBlobstore(blobstoreConfig config.BlobstoreConfig) routes.Blobstore {
	switch blobstoreConfig.BlobstoreType {
	case "local", "LOCAL":
		fmt.Println("Creating local blobstore", "path prefix:", blobstoreConfig.LocalConfig.PathPrefix)
		return local_blobstore.NewLocalBlobstore(blobstoreConfig.LocalConfig.PathPrefix)

	case "s3", "S3", "AWS", "aws":
		return s3_blobstore.NewS3NoRedirectBlobStore(*blobstoreConfig.S3Config)
	default:
		log.Log.Fatal("blobstoreConfig is invalid. BlobstoreType missing.")
		return nil // satisfy compiler
	}
}

func usesLocalBlobstore(config config.Config) bool {
	return config.Packages.BlobstoreType == "local" ||
		config.Buildpacks.BlobstoreType == "local" ||
		config.Droplets.BlobstoreType == "local"
}
