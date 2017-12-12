package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/blobstores/azure"
	"github.com/petergtz/bitsgo/blobstores/decorator"
	"github.com/petergtz/bitsgo/blobstores/gcp"
	"github.com/petergtz/bitsgo/blobstores/local"
	"github.com/petergtz/bitsgo/blobstores/openstack"
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

	appStashBlobstore := createAppStashBlobstore(config.AppStash)
	packageBlobstore, signPackageURLHandler := createBlobstoreAndSignURLHandler(config.Packages, config.PublicEndpointUrl().Host, config.Port, config.Secret, "packages")
	dropletBlobstore, signDropletURLHandler := createBlobstoreAndSignURLHandler(config.Droplets, config.PublicEndpointUrl().Host, config.Port, config.Secret, "droplets")
	buildpackBlobstore, signBuildpackURLHandler := createBlobstoreAndSignURLHandler(config.Buildpacks, config.PublicEndpointUrl().Host, config.Port, config.Secret, "buildpacks")
	buildpackCacheBlobstore, signBuildpackCacheURLHandler := createBuildpackCacheSignURLHandler(config.Droplets, config.PublicEndpointUrl().Host, config.Port, config.Secret, "droplets")

	metricsService := statsd.NewMetricsService()

	appstashHandler := bitsgo.NewAppStashHandler(appStashBlobstore, config.AppStash.MaxBodySizeBytes())
	packageHandler := bitsgo.NewResourceHandler(packageBlobstore, "package", metricsService, config.Packages.MaxBodySizeBytes())
	buildpackHandler := bitsgo.NewResourceHandler(buildpackBlobstore, "buildpack", metricsService, config.Buildpacks.MaxBodySizeBytes())
	dropletHandler := bitsgo.NewResourceHandler(dropletBlobstore, "droplet", metricsService, config.Droplets.MaxBodySizeBytes())
	buildpackCacheHandler := bitsgo.NewResourceHandler(buildpackCacheBlobstore, "buildpack_cache", metricsService, config.Droplets.MaxBodySizeBytes())

	rootRouter := mux.NewRouter()
	rootRouter.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	internalRouter := mux.NewRouter()
	rootRouter.Host(config.PrivateEndpointUrl().Host).Handler(internalRouter)

	routes.SetUpSignRoute(internalRouter, middlewares.NewBasicAuthMiddleWare(basicAuthCredentialsFrom(config.SigningUsers)...),
		signPackageURLHandler, signDropletURLHandler, signBuildpackURLHandler, signBuildpackCacheURLHandler)

	routes.SetUpAppStashRoutes(internalRouter, appstashHandler)
	routes.SetUpPackageRoutes(internalRouter, packageHandler)
	routes.SetUpBuildpackRoutes(internalRouter, buildpackHandler)
	routes.SetUpDropletRoutes(internalRouter, dropletHandler)
	routes.SetUpBuildpackCacheRoutes(internalRouter, buildpackCacheHandler)

	publicRouter := mux.NewRouter()
	rootRouter.Host(config.PublicEndpointUrl().Host).Handler(negroni.New(
		&local.SignatureVerificationMiddleware{&pathsigner.PathSignerValidator{config.Secret, clock.New()}},
		negroni.Wrap(publicRouter),
	))
	routes.SetUpPackageRoutes(publicRouter, packageHandler)
	routes.SetUpBuildpackRoutes(publicRouter, buildpackHandler)
	routes.SetUpDropletRoutes(publicRouter, dropletHandler)
	routes.SetUpBuildpackCacheRoutes(publicRouter, buildpackCacheHandler)

	httpServer := &http.Server{
		Handler: negroni.New(
			middlewares.NewMetricsMiddleware(metricsService),
			middlewares.NewZapLoggerMiddleware(log.Log),
			negroni.Wrap(rootRouter)),
		Addr:         fmt.Sprintf("0.0.0.0:%v", config.Port),
		WriteTimeout: 60 * time.Minute,
		ReadTimeout:  60 * time.Minute,
	}

	e = httpServer.ListenAndServe()
	log.Log.Fatalw("http server crashed", "error", e)
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
	localResourceSigner := createLocalResourceSigner(publicHost, port, secret, resourceType)
	switch strings.ToLower(blobstoreConfig.BlobstoreType) {
	case "local":
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
				local.NewBlobstore(*blobstoreConfig.LocalConfig)),
			createLocalSignResourceHandler(publicHost, port, secret, resourceType)
	case "s3", "aws":
		log.Log.Infow("Creating S3 blobstore", "bucket", blobstoreConfig.S3Config.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				s3.NewBlobstore(*blobstoreConfig.S3Config)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					s3.NewBlobstore(*blobstoreConfig.S3Config)),
				localResourceSigner)
	case "google", "gcp":
		log.Log.Infow("Creating GCP blobstore", "bucket", blobstoreConfig.GCPConfig.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				gcp.NewBlobstore(*blobstoreConfig.GCPConfig)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					gcp.NewBlobstore(*blobstoreConfig.GCPConfig)),
				localResourceSigner)
	case "azure":
		log.Log.Infow("Creating Azure blobstore", "container", blobstoreConfig.AzureConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				azure.NewBlobstore(*blobstoreConfig.AzureConfig)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					azure.NewBlobstore(*blobstoreConfig.AzureConfig)),
				localResourceSigner)
	case "openstack":
		log.Log.Infow("Creating Openstack blobstore", "container", blobstoreConfig.OpenstackConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig)),
				localResourceSigner)
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
						webdav.NewBlobstore(*blobstoreConfig.WebdavConfig), resourceType+"/")),
				localResourceSigner)

	default:
		log.Log.Fatalw("blobstoreConfig is invalid.", "blobstore-type", blobstoreConfig.BlobstoreType)
		return nil, nil // satisfy compiler
	}
}

func createBuildpackCacheSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicHost string, port int, secret string, resourceType string) (bitsgo.Blobstore, *bitsgo.SignResourceHandler) {
	localResourceSigner := createLocalResourceSigner(publicHost, port, secret, resourceType)
	switch strings.ToLower(blobstoreConfig.BlobstoreType) {
	case "local":
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					local.NewBlobstore(*blobstoreConfig.LocalConfig),
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
						s3.NewBlobstore(*blobstoreConfig.S3Config),
						"buildpack_cache")),
				localResourceSigner)
	case "gcp", "google":
		log.Log.Infow("Creating GCP blobstore", "bucket", blobstoreConfig.GCPConfig.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					gcp.NewBlobstore(*blobstoreConfig.GCPConfig),
					"buildpack_cache/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						gcp.NewBlobstore(*blobstoreConfig.GCPConfig),
						"buildpack_cache")),
				localResourceSigner)
	case "azure":
		log.Log.Infow("Creating Azure blobstore", "container", blobstoreConfig.AzureConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					azure.NewBlobstore(*blobstoreConfig.AzureConfig),
					"buildpack_cache/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						azure.NewBlobstore(*blobstoreConfig.AzureConfig),
						"buildpack_cache")),
				localResourceSigner)
	case "openstack":
		log.Log.Infow("Creating Openstack blobstore", "container", blobstoreConfig.OpenstackConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig),
					"buildpack_cache/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig),
						"buildpack_cache")),
				localResourceSigner)
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
						webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
						"buildpack_cache")),
				localResourceSigner)
	default:
		log.Log.Fatalw("blobstoreConfig is invalid.", "blobstore-type", blobstoreConfig.BlobstoreType)
		return nil, nil // satisfy compiler
	}
}

func createLocalSignResourceHandler(publicHost string, port int, secret string, resourceType string) *bitsgo.SignResourceHandler {
	signer := createLocalResourceSigner(publicHost, port, secret, resourceType)
	return bitsgo.NewSignResourceHandler(signer, signer)
}

func createLocalResourceSigner(publicHost string, port int, secret string, resourceType string) bitsgo.ResourceSigner {
	return &local.LocalResourceSigner{
		DelegateEndpoint:   fmt.Sprintf("http://%v:%v", publicHost, port),
		Signer:             &pathsigner.PathSignerValidator{secret, clock.New()},
		ResourcePathPrefix: "/" + resourceType + "/",
	}
}

func createAppStashBlobstore(blobstoreConfig config.BlobstoreConfig) bitsgo.NoRedirectBlobstore {
	switch strings.ToLower(blobstoreConfig.BlobstoreType) {
	case "local":
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
			local.NewBlobstore(*blobstoreConfig.LocalConfig))
	case "s3", "aws":
		log.Log.Infow("Creating S3 blobstore", "bucket", blobstoreConfig.S3Config.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
			s3.NewBlobstore(*blobstoreConfig.S3Config))
	case "gcp", "google":
		log.Log.Infow("Creating GCP blobstore", "bucket", blobstoreConfig.GCPConfig.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
			gcp.NewBlobstore(*blobstoreConfig.GCPConfig))
	case "azure":
		log.Log.Infow("Creating Azure blobstore", "container", blobstoreConfig.AzureConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
			azure.NewBlobstore(*blobstoreConfig.AzureConfig))
	case "openstack":
		log.Log.Infow("Creating Openstack blobstore", "container", blobstoreConfig.OpenstackConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
			openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig))
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
