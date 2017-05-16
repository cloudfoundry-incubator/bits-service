package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/blobstores/decorator"
	"github.com/petergtz/bitsgo/blobstores/local"
	"github.com/petergtz/bitsgo/blobstores/s3"
	"github.com/petergtz/bitsgo/blobstores/webdav"
	"github.com/petergtz/bitsgo/config"
	log "github.com/petergtz/bitsgo/logger"
	"github.com/petergtz/bitsgo/middlewares"
	"github.com/petergtz/bitsgo/pathsigner"
	"github.com/petergtz/bitsgo/routes"
	"github.com/petergtz/bitsgo/statsd"
	"github.com/urfave/negroni"
	"go.uber.org/zap"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	configPath = kingpin.Flag("config", "specify config to use").Required().Short('c').String()
)

func main() {
	kingpin.Parse()

	config, e := config.LoadConfig(*configPath)

	if e != nil {
		log.Log.Fatal("Could not load config.", "error", e)
	}
	log.Log.Infow("Logging level", "log-level", config.Logging.Level)
	log.SetLogger(createLoggerWith(config.Logging.Level))

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

	routes.SetUpSignRoute(internalRouter, middlewares.NewBasicAuthMiddleWare(basicAuthCredentialsFrom(config.SigningUsers)...),
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

	httpHandler := negroni.New(
		middlewares.NewMetricsMiddleware(statsd.NewMetricsService()),
		middlewares.NewZapLoggerMiddleware(log.Log))
	if config.MaxBodySizeBytes() != 0 {
		httpHandler.Use(middlewares.NewBodySizeLimitMiddleware(config.MaxBodySizeBytes()))
	}
	httpHandler.UseHandler(rootRouter)

	httpServer := &http.Server{
		Handler:      httpHandler,
		Addr:         fmt.Sprintf("0.0.0.0:%v", config.Port),
		WriteTimeout: 60 * time.Minute,
		ReadTimeout:  60 * time.Minute,
	}

	e = httpServer.ListenAndServe()
	log.Log.Fatal("http server crashed", "error", e)
}

func createLoggerWith(logLevel string) *zap.Logger {
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zapLogLevelFrom(logLevel)
	logger, e := loggerConfig.Build()
	if e != nil {
		log.Log.Panic(e)
	}
	return logger
}

func zapLogLevelFrom(configLogLevel string) zap.AtomicLevel {
	switch strings.ToLower(configLogLevel) {
	case "", "debug":
		return zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		return zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		return zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "fatal":
		return zap.NewAtomicLevelAt(zap.FatalLevel)
	default:
		log.Log.Fatal("Invalid log level in config", "log-level", configLogLevel)
		return zap.NewAtomicLevelAt(-1)
	}
}

func basicAuthCredentialsFrom(configCredententials []config.Credential) (basicAuthCredentials []middlewares.Credential) {
	basicAuthCredentials = make([]middlewares.Credential, len(configCredententials))
	for i := range configCredententials {
		basicAuthCredentials[i] = middlewares.Credential(configCredententials[i])
	}
	return
}

func createBlobstoreAndSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicHost string, port int, secret string, resourceType string) (bitsgo.Blobstore, *bitsgo.SignResourceHandler) {
	switch strings.ToLower(blobstoreConfig.BlobstoreType) {
	case "local":
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
				local.NewBlobstore(blobstoreConfig.LocalConfig.PathPrefix)),
			createLocalSignResourceHandler(publicHost, port, secret, resourceType)
	case "s3", "aws":
		log.Log.Infow("Creating S3 blobstore", "bucket", blobstoreConfig.S3Config.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				s3.NewBlobstore(*blobstoreConfig.S3Config)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					s3.NewResourceSigner(*blobstoreConfig.S3Config)))
	case "webdav":
		log.Log.Infow("Creating Webdav blobstore",
			"public-endpoint", blobstoreConfig.WebdavConfig.PublicEndpoint,
			"private-endpoint", blobstoreConfig.WebdavConfig.PrivateEndpoint)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
					resourceType+"/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						webdav.NewWebdavResourceSigner(*blobstoreConfig.WebdavConfig), resourceType+"/")))

	default:
		log.Log.Fatalw("blobstoreConfig is invalid.", "blobstore-type", blobstoreConfig.BlobstoreType)
		return nil, nil // satisfy compiler
	}
}

func createBuildpackCacheSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicHost string, port int, secret string, resourceType string) (bitsgo.Blobstore, *bitsgo.SignResourceHandler) {
	switch strings.ToLower(blobstoreConfig.BlobstoreType) {
	case "local":
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					local.NewBlobstore(blobstoreConfig.LocalConfig.PathPrefix),
					"buildpack_cache/")),
			createLocalSignResourceHandler(publicHost, port, secret, resourceType)
	case "s3", "aws":
		log.Log.Infow("Creating S3 blobstore", "bucket", blobstoreConfig.S3Config.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					s3.NewBlobstore(*blobstoreConfig.S3Config),
					"buildpack_cache/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						s3.NewResourceSigner(*blobstoreConfig.S3Config),
						"buildpack_cache")))
	case "webdav":
		log.Log.Infow("Creating Webdav blobstore",
			"public-endpoint", blobstoreConfig.WebdavConfig.PublicEndpoint,
			"private-endpoint", blobstoreConfig.WebdavConfig.PrivateEndpoint)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
					resourceType+"/buildpack_cache/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						webdav.NewWebdavResourceSigner(*blobstoreConfig.WebdavConfig),
						"buildpack_cache")))
	default:
		log.Log.Fatalw("blobstoreConfig is invalid.", "blobstore-type", blobstoreConfig.BlobstoreType)
		return nil, nil // satisfy compiler
	}
}

func createLocalSignResourceHandler(publicHost string, port int, secret string, resourceType string) *bitsgo.SignResourceHandler {
	return bitsgo.NewSignResourceHandler(&local.LocalResourceSigner{
		DelegateEndpoint:   fmt.Sprintf("http://%v:%v", publicHost, port),
		Signer:             &pathsigner.PathSignerValidator{secret, clock.New()},
		ResourcePathPrefix: "/" + resourceType + "/",
		Clock:              clock.New(),
	})
}

func createAppStashBlobstore(blobstoreConfig config.BlobstoreConfig) bitsgo.NoRedirectBlobstore {
	switch strings.ToLower(blobstoreConfig.BlobstoreType) {
	case "local":
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
			local.NewBlobstore(blobstoreConfig.LocalConfig.PathPrefix))

	case "s3", "aws":
		log.Log.Infow("Creating S3 blobstore", "bucket", blobstoreConfig.S3Config.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
			s3.NewBlobstore(*blobstoreConfig.S3Config))
	case "webdav":
		log.Log.Infow("Creating Webdav blobstore",
			"public-endpoint", blobstoreConfig.WebdavConfig.PublicEndpoint,
			"private-endpoint", blobstoreConfig.WebdavConfig.PrivateEndpoint)
		return decorator.ForBlobstoreWithPathPartitioning(
			decorator.ForBlobstoreWithPathPrefixing(
				webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
				"app_stash/"))
	default:
		log.Log.Fatalw("blobstoreConfig is invalid.", "blobstore-type", blobstoreConfig.BlobstoreType)
		return nil // satisfy compiler
	}
}
