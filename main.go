package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo/basic_auth_middleware"
	"github.com/petergtz/bitsgo/blobstores/decorator"
	"github.com/petergtz/bitsgo/blobstores/local"
	"github.com/petergtz/bitsgo/blobstores/s3"
	"github.com/petergtz/bitsgo/config"
	log "github.com/petergtz/bitsgo/logger"
	"github.com/petergtz/bitsgo/middlewares"
	"github.com/petergtz/bitsgo/pathsigner"
	"github.com/petergtz/bitsgo/routes"
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
	log.Log.Info("Logging level", zap.String("log-level", config.Logging.Level))

	log.SetLogger(zap.New(zap.NewTextEncoder(), zapLogLevelFrom(config.Logging.Level), zap.AddCaller()))

	rootRouter := mux.NewRouter()
	rootRouter.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	internalRouter := mux.NewRouter()

	rootRouter.Host(config.PrivateEndpointUrl().Host).Handler(internalRouter)

	appStashBlobstore := createAppStashBlobstore(config.AppStash)
	packageBlobstore, signPackageURLHandler := createBlobstoreAndSignURLHandler(config.Packages, config.PublicEndpointUrl().Host, config.Port, config.Secret, "packages")
	dropletBlobstore, signDropletURLHandler := createBlobstoreAndSignURLHandler(config.Droplets, config.PublicEndpointUrl().Host, config.Port, config.Secret, "droplets")
	buildpackBlobstore, signBuildpackURLHandler := createBlobstoreAndSignURLHandler(config.Buildpacks, config.PublicEndpointUrl().Host, config.Port, config.Secret, "buildpacks")
	buildpackCacheBlobstore, signBuildpackCacheURLHandler := createBuildpackCacheSignURLHandler(config.Droplets, config.PublicEndpointUrl().Host, config.Port, config.Secret, "droplets")

	routes.SetUpSignRoute(internalRouter, basic_auth_middleware.NewBasicAuthMiddleWare(basicAuthCredentialsFrom(config.SigningUsers)...),
		signPackageURLHandler, signDropletURLHandler, signBuildpackURLHandler, signBuildpackCacheURLHandler)

	routes.SetUpAppStashRoutes(internalRouter, appStashBlobstore)
	routes.SetUpPackageRoutes(internalRouter, packageBlobstore)
	routes.SetUpBuildpackRoutes(internalRouter, buildpackBlobstore)
	routes.SetUpDropletRoutes(internalRouter, dropletBlobstore)
	routes.SetUpBuildpackCacheRoutes(internalRouter, buildpackCacheBlobstore)

	publicRouter := mux.NewRouter()
	rootRouter.Host(config.PublicEndpointUrl().Host).Handler(negroni.New(
		&local.SignatureVerificationMiddleware{&pathsigner.PathSignerValidator{config.Secret, clock.New()}},
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
		WriteTimeout: 60 * time.Minute,
		ReadTimeout:  60 * time.Minute,
	}

	e = httpServer.ListenAndServe()
	log.Log.Fatal("http server crashed", zap.Error(e))
}

func zapLogLevelFrom(configLogLevel string) zap.Level {
	switch strings.ToLower(configLogLevel) {
	case "", "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "fatal":
		return zap.FatalLevel
	default:
		log.Log.Fatal("Invalid log level in config", zap.String("log-level", configLogLevel))
		return -1
	}
}

func basicAuthCredentialsFrom(configCredententials []config.Credential) (basicAuthCredentials []basic_auth_middleware.Credential) {
	basicAuthCredentials = make([]basic_auth_middleware.Credential, len(configCredententials))
	for i := range configCredententials {
		basicAuthCredentials[i].Username = configCredententials[i].Username
		basicAuthCredentials[i].Password = configCredententials[i].Password
	}
	return
}

func createBlobstoreAndSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicHost string, port int, secret string, resourceType string) (routes.Blobstore, *routes.SignResourceHandler) {
	switch strings.ToLower(blobstoreConfig.BlobstoreType) {
	case "local":
		log.Log.Info("Creating local blobstore", zap.String("path-prefix", blobstoreConfig.LocalConfig.PathPrefix))
		return decorator.ForBlobstoreWithPathPartitioning(
				local.NewBlobstore(blobstoreConfig.LocalConfig.PathPrefix)),
			createLocalSignResourceHandler(publicHost, port, secret, resourceType)
	case "s3", "aws":
		log.Log.Info("Creating S3 blobstore", zap.String("bucket", blobstoreConfig.S3Config.Bucket))
		return decorator.ForBlobstoreWithPathPartitioning(
				s3.NewLegacyBlobstore(*blobstoreConfig.S3Config)),
			routes.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					s3.NewS3ResourceSigner(*blobstoreConfig.S3Config)))
	default:
		log.Log.Fatal("blobstoreConfig is invalid.", zap.String("blobstore-type", blobstoreConfig.BlobstoreType))
		return nil, nil // satisfy compiler
	}
}

func createBuildpackCacheSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicHost string, port int, secret string, resourceType string) (routes.Blobstore, *routes.SignResourceHandler) {
	switch strings.ToLower(blobstoreConfig.BlobstoreType) {
	case "local":
		log.Log.Info("Creating local blobstore", zap.String("path-prefix", blobstoreConfig.LocalConfig.PathPrefix))
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					local.NewBlobstore(blobstoreConfig.LocalConfig.PathPrefix),
					"buildpack_cache/")),
			createLocalSignResourceHandler(publicHost, port, secret, resourceType)
	case "s3", "aws":
		log.Log.Info("Creating S3 blobstore", zap.String("bucket", blobstoreConfig.S3Config.Bucket))
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					s3.NewLegacyBlobstore(*blobstoreConfig.S3Config),
					"buildpack_cache/")),
			routes.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.DecorateWithPrefixingPathResourceSigner(
						s3.NewS3ResourceSigner(*blobstoreConfig.S3Config),
						"buildpack_cache")))
	default:
		log.Log.Fatal("blobstoreConfig is invalid.", zap.String("blobstore-type", blobstoreConfig.BlobstoreType))
		return nil, nil // satisfy compiler
	}
}

func createLocalSignResourceHandler(publicHost string, port int, secret string, resourceType string) *routes.SignResourceHandler {
	return routes.NewSignResourceHandler(&local.LocalResourceSigner{
		DelegateEndpoint:   fmt.Sprintf("http://%v:%v", publicHost, port),
		Signer:             &pathsigner.PathSignerValidator{secret, clock.New()},
		ResourcePathPrefix: "/" + resourceType + "/",
		Clock:              clock.New(),
	})
}

func createAppStashBlobstore(blobstoreConfig config.BlobstoreConfig) routes.Blobstore {
	switch strings.ToLower(blobstoreConfig.BlobstoreType) {
	case "local":
		log.Log.Info("Creating local blobstore", zap.String("path-prefix", blobstoreConfig.LocalConfig.PathPrefix))
		return decorator.ForBlobstoreWithPathPartitioning(
			local.NewBlobstore(blobstoreConfig.LocalConfig.PathPrefix))

	case "s3", "aws":
		log.Log.Info("Creating S3 blobstore", zap.String("bucket", blobstoreConfig.S3Config.Bucket))
		return decorator.ForBlobstoreWithPathPartitioning(
			s3.NewNoRedirectBlobStore(*blobstoreConfig.S3Config))
	default:
		log.Log.Fatal("blobstoreConfig is invalid.", zap.String("blobstore-type", blobstoreConfig.BlobstoreType))
		return nil // satisfy compiler
	}
}
