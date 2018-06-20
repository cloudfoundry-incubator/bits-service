package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/petergtz/bitsgo/ccupdater"

	"github.com/benbjohnson/clock"
	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/blobstores/alibaba"
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
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	configPath = kingpin.Flag("config", "specify config to use").Required().Short('c').String()
)

func main() {
	kingpin.Parse()

	config, e := config.LoadConfig(*configPath)

	if e != nil {
		log.Log.Fatalw("Could not load config.", "error", e)
	}
	log.Log.Infow("Logging level", "log-level", config.Logging.Level)
	logger := createLoggerWith(config.Logging.Level)
	log.SetLogger(logger)

	appStashBlobstore, signAppStashURLHandler := createAppStashBlobstore(config.AppStash, config.PublicEndpointUrl(), config.Port, config.Secret)
	packageBlobstore, signPackageURLHandler := createBlobstoreAndSignURLHandler(config.Packages, config.PublicEndpointUrl(), config.Port, config.Secret, "packages")
	dropletBlobstore, signDropletURLHandler := createBlobstoreAndSignURLHandler(config.Droplets, config.PublicEndpointUrl(), config.Port, config.Secret, "droplets")
	buildpackBlobstore, signBuildpackURLHandler := createBlobstoreAndSignURLHandler(config.Buildpacks, config.PublicEndpointUrl(), config.Port, config.Secret, "buildpacks")
	buildpackCacheBlobstore, signBuildpackCacheURLHandler := createBuildpackCacheSignURLHandler(config.Droplets, config.PublicEndpointUrl(), config.Port, config.Secret, "droplets")

	metricsService := statsd.NewMetricsService()

	handler := routes.SetUpAllRoutes(
		config.PrivateEndpointUrl().Host,
		config.PublicEndpointUrl().Host,
		middlewares.NewBasicAuthMiddleWare(basicAuthCredentialsFrom(config.SigningUsers)...),
		&local.SignatureVerificationMiddleware{&pathsigner.PathSignerValidator{config.Secret, clock.New()}},
		signPackageURLHandler,
		signDropletURLHandler,
		signBuildpackURLHandler,
		signBuildpackCacheURLHandler,
		signAppStashURLHandler,
		bitsgo.NewAppStashHandlerWithSizeThresholds(appStashBlobstore, config.AppStash.MaxBodySizeBytes(), config.AppStashConfig.MinimumSizeBytes(), config.AppStashConfig.MaximumSizeBytes()),
		bitsgo.NewResourceHandlerWithUpdaterAndSizeThresholds(
			packageBlobstore,
			appStashBlobstore,
			createUpdater(config.CCUpdater),
			"package",
			metricsService,
			config.Packages.MaxBodySizeBytes(),
			config.AppStashConfig.MinimumSizeBytes(),
			config.AppStashConfig.MaximumSizeBytes(),
		),
		bitsgo.NewResourceHandler(buildpackBlobstore, appStashBlobstore, "buildpack", metricsService, config.Buildpacks.MaxBodySizeBytes()),
		bitsgo.NewResourceHandler(dropletBlobstore, appStashBlobstore, "droplet", metricsService, config.Droplets.MaxBodySizeBytes()),
		bitsgo.NewResourceHandler(buildpackCacheBlobstore, appStashBlobstore, "buildpack_cache", metricsService, config.BuildpackCache.MaxBodySizeBytes()))

	address := os.Getenv("BITS_LISTEN_ADDR")
	if address == "" {
		address = "0.0.0.0"
	}

	log.Log.Infow("Starting server",
		"ip-address", address,
		"port", config.Port,
		"public-endpoint", config.PublicEndpointUrl().Host,
		"private-endpoint", config.PrivateEndpointUrl().Host)
	httpServer := &http.Server{
		Handler: negroni.New(
			middlewares.NewMetricsMiddleware(metricsService),
			middlewares.NewZapLoggerMiddleware(log.Log),
			&middlewares.MultipartMiddleware{},
			negroni.Wrap(handler)),
		Addr:         fmt.Sprintf("%v:%v", address, config.Port),
		WriteTimeout: 60 * time.Minute,
		ReadTimeout:  60 * time.Minute,
		ErrorLog:     log.NewStdLog(logger),
	}
	e = httpServer.ListenAndServeTLS(config.CertFile, config.KeyFile)
	log.Log.Fatalw("http server crashed", "error", e)
}

func createLoggerWith(logLevel string) *zap.Logger {
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zapLogLevelFrom(logLevel)
	loggerConfig.DisableStacktrace = true
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

func createBlobstoreAndSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicEndpoint *url.URL, port int, secret string, resourceType string) (bitsgo.Blobstore, *bitsgo.SignResourceHandler) {
	localResourceSigner := createLocalResourceSigner(publicEndpoint, port, secret, resourceType)
	switch blobstoreConfig.BlobstoreType {
	case config.Local:
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
				local.NewBlobstore(*blobstoreConfig.LocalConfig)),
			bitsgo.NewSignResourceHandler(localResourceSigner, localResourceSigner)
	case config.AWS:
		log.Log.Infow("Creating S3 blobstore", "bucket", blobstoreConfig.S3Config.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				s3.NewBlobstore(*blobstoreConfig.S3Config)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					s3.NewBlobstore(*blobstoreConfig.S3Config)),
				localResourceSigner)
	case config.Google:
		log.Log.Infow("Creating GCP blobstore", "bucket", blobstoreConfig.GCPConfig.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				gcp.NewBlobstore(*blobstoreConfig.GCPConfig)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					gcp.NewBlobstore(*blobstoreConfig.GCPConfig)),
				localResourceSigner)
	case config.Azure:
		log.Log.Infow("Creating Azure blobstore", "container", blobstoreConfig.AzureConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				azure.NewBlobstore(*blobstoreConfig.AzureConfig)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					azure.NewBlobstore(*blobstoreConfig.AzureConfig)),
				localResourceSigner)
	case config.OpenStack:
		log.Log.Infow("Creating Openstack blobstore", "container", blobstoreConfig.OpenstackConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig)),
				localResourceSigner)
	case config.WebDAV:
		log.Log.Infow("Creating Webdav blobstore",
			"public-endpoint", blobstoreConfig.WebdavConfig.PublicEndpoint,
			"private-endpoint", blobstoreConfig.WebdavConfig.PrivateEndpoint)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
					blobstoreConfig.WebdavConfig.DirectoryKey+"/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
						blobstoreConfig.WebdavConfig.DirectoryKey+"/")),
				localResourceSigner)
	case config.Alibaba:
		log.Log.Infow("Creating Alibaba blobstore", "bucket", blobstoreConfig.AlibabaConfig.BucketName)
		return decorator.ForBlobstoreWithPathPartitioning(
				alibaba.NewBlobstore(*blobstoreConfig.AlibabaConfig)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					alibaba.NewBlobstore(*blobstoreConfig.AlibabaConfig)),
				localResourceSigner)
	default:
		log.Log.Fatalw("blobstoreConfig is invalid.", "blobstore-type", blobstoreConfig.BlobstoreType)
		return nil, nil // satisfy compiler
	}
}

func createBuildpackCacheSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicEndpoint *url.URL, port int, secret string, resourceType string) (bitsgo.Blobstore, *bitsgo.SignResourceHandler) {
	localResourceSigner := createLocalResourceSigner(publicEndpoint, port, secret, "buildpack_cache/entries")
	switch blobstoreConfig.BlobstoreType {
	case config.Local:
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					local.NewBlobstore(*blobstoreConfig.LocalConfig),
					"buildpack_cache/")),
			bitsgo.NewSignResourceHandler(localResourceSigner, localResourceSigner)
	case config.AWS:
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
	case config.Google:
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
	case config.Azure:
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
	case config.OpenStack:
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
	case config.WebDAV:
		log.Log.Infow("Creating Webdav blobstore",
			"public-endpoint", blobstoreConfig.WebdavConfig.PublicEndpoint,
			"private-endpoint", blobstoreConfig.WebdavConfig.PrivateEndpoint)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
					blobstoreConfig.WebdavConfig.DirectoryKey+"/buildpack_cache/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
						blobstoreConfig.WebdavConfig.DirectoryKey+"/buildpack_cache/")),
				localResourceSigner)
	case config.Alibaba:
		log.Log.Infow("Creating Alibaba blobstore", "bucket", blobstoreConfig.AlibabaConfig.BucketName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					alibaba.NewBlobstore(*blobstoreConfig.AlibabaConfig),
					"buildpack_cache/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						alibaba.NewBlobstore(*blobstoreConfig.AlibabaConfig),
						"buildpack_cache")),
				localResourceSigner)
	default:
		log.Log.Fatalw("blobstoreConfig is invalid.", "blobstore-type", blobstoreConfig.BlobstoreType)
		return nil, nil // satisfy compiler
	}
}

func createLocalResourceSigner(publicEndpoint *url.URL, port int, secret string, resourceType string) bitsgo.ResourceSigner {
	return &local.LocalResourceSigner{
		DelegateEndpoint:   fmt.Sprintf("%v://%v:%v", publicEndpoint.Scheme, publicEndpoint.Host, port),
		Signer:             &pathsigner.PathSignerValidator{secret, clock.New()},
		ResourcePathPrefix: "/" + resourceType + "/",
	}
}

func createAppStashBlobstore(blobstoreConfig config.BlobstoreConfig, publicEndpoint *url.URL, port int, secret string) (bitsgo.NoRedirectBlobstore, *bitsgo.SignResourceHandler) {
	signAppStashMatchesHandler := bitsgo.NewSignResourceHandler(
		nil, // signing for get is not necessary for app_stash
		&local.LocalResourceSigner{
			DelegateEndpoint:   fmt.Sprintf("%v://%v:%v", publicEndpoint.Scheme, publicEndpoint.Host, port),
			Signer:             &pathsigner.PathSignerValidator{secret, clock.New()},
			ResourcePathPrefix: "/app_stash/matches",
		})

	switch blobstoreConfig.BlobstoreType {
	case config.Local:
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					local.NewBlobstore(*blobstoreConfig.LocalConfig),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	case config.AWS:
		log.Log.Infow("Creating S3 blobstore", "bucket", blobstoreConfig.S3Config.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					s3.NewBlobstore(*blobstoreConfig.S3Config),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	case config.Google:
		log.Log.Infow("Creating GCP blobstore", "bucket", blobstoreConfig.GCPConfig.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					gcp.NewBlobstore(*blobstoreConfig.GCPConfig),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	case config.Azure:
		log.Log.Infow("Creating Azure blobstore", "container", blobstoreConfig.AzureConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					azure.NewBlobstore(*blobstoreConfig.AzureConfig),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	case config.OpenStack:
		log.Log.Infow("Creating Openstack blobstore", "container", blobstoreConfig.OpenstackConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	case config.WebDAV:
		log.Log.Infow("Creating Webdav blobstore",
			"public-endpoint", blobstoreConfig.WebdavConfig.PublicEndpoint,
			"private-endpoint", blobstoreConfig.WebdavConfig.PrivateEndpoint)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
					blobstoreConfig.WebdavConfig.DirectoryKey+"/app_bits_cache/")),
			signAppStashMatchesHandler
	case config.Alibaba:
		log.Log.Infow("Creating Alibaba blobstore", "bucket-name", blobstoreConfig.AlibabaConfig.BucketName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					alibaba.NewBlobstore(*blobstoreConfig.AlibabaConfig),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	default:
		log.Log.Fatalw("blobstoreConfig is invalid.", "blobstore-type", blobstoreConfig.BlobstoreType)
		return nil, nil // satisfy compiler
	}
}

func createUpdater(ccUpdaterConfig *config.CCUpdaterConfig) bitsgo.Updater {
	if ccUpdaterConfig == nil {
		return &bitsgo.NullUpdater{}
	}
	return ccupdater.NewCCUpdater(
		ccUpdaterConfig.Endpoint,
		ccUpdaterConfig.Method,
		ccUpdaterConfig.ClientCertFile,
		ccUpdaterConfig.ClientKeyFile,
		ccUpdaterConfig.CACertFile)
}
