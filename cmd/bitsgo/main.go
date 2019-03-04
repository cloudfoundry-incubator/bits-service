package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/bits-service/ccupdater"
	"github.com/cloudfoundry-incubator/bits-service/oci_registry"

	"github.com/benbjohnson/clock"
	bitsgo "github.com/cloudfoundry-incubator/bits-service"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/alibaba"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/azure"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/decorator"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/gcp"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/local"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/openstack"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/s3"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/webdav"
	"github.com/cloudfoundry-incubator/bits-service/config"
	log "github.com/cloudfoundry-incubator/bits-service/logger"
	"github.com/cloudfoundry-incubator/bits-service/middlewares"
	"github.com/cloudfoundry-incubator/bits-service/pathsigner"
	"github.com/cloudfoundry-incubator/bits-service/routes"
	"github.com/cloudfoundry-incubator/bits-service/statsd"
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

	if config.Secret != "" {
		log.Log.Infow("Config file uses deprecated \"secret\" property. Please consider using \"signing_keys\" instead.")
	}

	metricsService := statsd.NewMetricsService()

	appStashBlobstore, signAppStashURLHandler := createAppStashBlobstore(config.AppStash, config.PublicEndpointUrl(), config.Port, config.Secret, config.SigningKeysMap(), config.ActiveKeyID, log.Log, metricsService)
	packageBlobstore, signPackageURLHandler := createBlobstoreAndSignURLHandler(config.Packages, config.PublicEndpointUrl(), config.Port, config.Secret, config.SigningKeysMap(), config.ActiveKeyID, "packages", log.Log, metricsService)
	dropletBlobstore, signDropletURLHandler := createBlobstoreAndSignURLHandler(config.Droplets, config.PublicEndpointUrl(), config.Port, config.Secret, config.SigningKeysMap(), config.ActiveKeyID, "droplets", log.Log, metricsService)
	buildpackBlobstore, signBuildpackURLHandler := createBlobstoreAndSignURLHandler(config.Buildpacks, config.PublicEndpointUrl(), config.Port, config.Secret, config.SigningKeysMap(), config.ActiveKeyID, "buildpacks", log.Log, metricsService)
	buildpackCacheBlobstore, signBuildpackCacheURLHandler := createBuildpackCacheSignURLHandler(config.Droplets, config.PublicEndpointUrl(), config.Port, config.Secret, config.SigningKeysMap(), config.ActiveKeyID, log.Log, metricsService)

	go regularlyEmitGoRoutines(metricsService)

	var (
		ociImageHandler      *oci_registry.ImageHandler
		registryEndpointHost = ""
	)
	if config.EnableRegistry {
		ociImageHandler = &oci_registry.ImageHandler{
			ImageManager: oci_registry.NewBitsImageManager(
				createRootFSBlobstore(config.RootFS),
				dropletBlobstore,
				// TODO: We should use a differently decorated blobstore for digestLookupStore:
				// We want one with a non-partitioned prefix, so real droplets and
				// oci-droplet layers (i.e. droplets with adjusted path prefixes)
				// are easily distinguishable from their paths in the blobstore.
				dropletBlobstore,
			),
		}
		registryEndpointHost = config.RegistryEndpointUrl().Host
		log.Log.Infow("Starting with OCI image registry",
			"registry-host", registryEndpointHost,
			"http-enabled", config.HttpEnabled,
			"http-port", config.HttpPort,
			"https-port", config.Port,
		)
	}

	handler := routes.SetUpAllRoutes(
		config.PrivateEndpointUrl().Host,
		config.PublicEndpointUrl().Host,
		registryEndpointHost,
		middlewares.NewBasicAuthMiddleWare(basicAuthCredentialsFrom(config.SigningUsers)...),
		&middlewares.SignatureVerificationMiddleware{pathsigner.Validate(&pathsigner.PathSignerValidator{
			config.Secret,
			clock.New(),
			config.SigningKeysMap(),
			config.ActiveKeyID,
		})},
		signPackageURLHandler,
		signDropletURLHandler,
		signBuildpackURLHandler,
		signBuildpackCacheURLHandler,
		signAppStashURLHandler,
		bitsgo.NewAppStashHandlerWithSizeThresholds(appStashBlobstore, config.AppStash.MaxBodySizeBytes(), config.AppStashConfig.MinimumSizeBytes(), config.AppStashConfig.MaximumSizeBytes(), metricsService),
		bitsgo.NewResourceHandlerWithUpdaterAndSizeThresholds(
			packageBlobstore,
			appStashBlobstore,
			createUpdater(config.CCUpdater),
			"package",
			metricsService,
			config.Packages.MaxBodySizeBytes(),
			config.AppStashConfig.MinimumSizeBytes(),
			config.AppStashConfig.MaximumSizeBytes(),
			config.ShouldProxyGetRequests,
		),
		bitsgo.NewResourceHandler(buildpackBlobstore, appStashBlobstore, "buildpack", metricsService, config.Buildpacks.MaxBodySizeBytes(), config.ShouldProxyGetRequests),
		bitsgo.NewResourceHandler(dropletBlobstore, appStashBlobstore, "droplet", metricsService, config.Droplets.MaxBodySizeBytes(), config.ShouldProxyGetRequests),
		bitsgo.NewResourceHandler(buildpackCacheBlobstore, appStashBlobstore, "buildpack_cache", metricsService, config.BuildpackCache.MaxBodySizeBytes(), config.ShouldProxyGetRequests),
		ociImageHandler,
	)

	address := os.Getenv("BITS_LISTEN_ADDR")
	if address == "" {
		address = "0.0.0.0"
	}

	httpServer := &http.Server{
		Handler: negroni.New(
			middlewares.NewMetricsMiddleware(metricsService),
			middlewares.NewZapLoggerMiddleware(log.Log),
			&middlewares.PanicMiddleware{},
			&middlewares.MultipartMiddleware{},
			negroni.Wrap(handler)),
		WriteTimeout: 60 * time.Minute,
		ReadTimeout:  60 * time.Minute,
		ErrorLog:     log.NewStdLog(logger),
	}
	if config.HttpEnabled {
		go listenAndServe(httpServer, address, config)
	}
	listenAndServeTLS(httpServer, address, config)
}

func listenAndServe(httpServer *http.Server, address string, c config.Config) {
	httpServer.Addr = fmt.Sprintf("%v:%v", address, c.HttpPort)
	log.Log.Infow("Starting HTTP server",
		"ip-address", address,
		"port", c.HttpPort,
		"public-endpoint", c.PublicEndpointUrl().Host,
		"private-endpoint", c.PrivateEndpointUrl().Host)
	e := httpServer.ListenAndServe()
	log.Log.Fatalw("HTTP server crashed", "error", e)
}

func listenAndServeTLS(httpServer *http.Server, address string, c config.Config) {
	httpServer.Addr = fmt.Sprintf("%v:%v", address, c.Port)
	// TLSConfig taken from https://blog.cloudflare.com/exposing-go-on-the-internet/
	httpServer.TLSConfig = &tls.Config{
		PreferServerCipherSuites: true,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519,
		},
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,

			// Best disabled, as they don't provide Forward Secrecy,
			// but might be necessary for some clients
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,

			// These are in the golang default cipher suite as well (disabled for now)
			// tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			// tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			// tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			// tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		},
	}

	log.Log.Infow("Starting HTTPS server",
		"ip-address", address,
		"port", c.Port,
		"public-endpoint", c.PublicEndpointUrl().Host,
		"private-endpoint", c.PrivateEndpointUrl().Host)
	e := httpServer.ListenAndServeTLS(c.CertFile, c.KeyFile)
	log.Log.Fatalw("HTTPS server crashed", "error", e)
}

func createLoggerWith(logLevel string) *zap.Logger {
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zapLogLevelFrom(logLevel)
	loggerConfig.DisableStacktrace = true
	loggerConfig.Sampling = nil
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

func createBlobstoreAndSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicEndpoint *url.URL, port int, secret string, signingKeys map[string]string, activeKeyID string, resourceType string, logger *zap.SugaredLogger, metricsService bitsgo.MetricsService) (bitsgo.Blobstore, *bitsgo.SignResourceHandler) {
	localResourceSigner := createLocalResourceSigner(publicEndpoint, port, secret, signingKeys, activeKeyID, resourceType)
	switch blobstoreConfig.BlobstoreType {
	case config.Local:
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithMetricsEmitter(
					local.NewBlobstore(*blobstoreConfig.LocalConfig),
					metricsService,
					resourceType)),
			bitsgo.NewSignResourceHandler(localResourceSigner, localResourceSigner)
	case config.AWS:
		log.Log.Infow("Creating S3 blobstore", "bucket", blobstoreConfig.S3Config.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithMetricsEmitter(
					s3.NewBlobstoreWithLogger(*blobstoreConfig.S3Config, logger),
					metricsService,
					resourceType)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					s3.NewBlobstoreWithLogger(*blobstoreConfig.S3Config, logger)),
				localResourceSigner)
	case config.Google:
		log.Log.Infow("Creating GCP blobstore", "bucket", blobstoreConfig.GCPConfig.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithMetricsEmitter(
					gcp.NewBlobstore(*blobstoreConfig.GCPConfig),
					metricsService,
					resourceType)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					gcp.NewBlobstore(*blobstoreConfig.GCPConfig)),
				localResourceSigner)
	case config.Azure:
		log.Log.Infow("Creating Azure blobstore", "container", blobstoreConfig.AzureConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithMetricsEmitter(
					azure.NewBlobstore(*blobstoreConfig.AzureConfig, metricsService),
					metricsService,
					resourceType)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					azure.NewBlobstore(*blobstoreConfig.AzureConfig, metricsService)),
				localResourceSigner)
	case config.OpenStack:
		log.Log.Infow("Creating Openstack blobstore", "container", blobstoreConfig.OpenstackConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithMetricsEmitter(
					openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig),
					metricsService,
					resourceType)),
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
					decorator.ForBlobstoreWithMetricsEmitter(
						webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
						metricsService,
						resourceType),
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
				decorator.ForBlobstoreWithMetricsEmitter(
					alibaba.NewBlobstore(*blobstoreConfig.AlibabaConfig),
					metricsService,
					resourceType)),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					alibaba.NewBlobstore(*blobstoreConfig.AlibabaConfig)),
				localResourceSigner)
	default:
		log.Log.Fatalw("blobstoreConfig is invalid.", "blobstore-type", blobstoreConfig.BlobstoreType)
		return nil, nil // satisfy compiler
	}
}

func createBuildpackCacheSignURLHandler(blobstoreConfig config.BlobstoreConfig, publicEndpoint *url.URL, port int, secret string, signingKeys map[string]string, activeKeyID string, logger *zap.SugaredLogger, metricsService bitsgo.MetricsService) (bitsgo.Blobstore, *bitsgo.SignResourceHandler) {
	localResourceSigner := createLocalResourceSigner(publicEndpoint, port, secret, signingKeys, activeKeyID, "buildpack_cache/entries")
	switch blobstoreConfig.BlobstoreType {
	case config.Local:
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						local.NewBlobstore(*blobstoreConfig.LocalConfig),
						metricsService,
						"buildpack_cache"),
					"buildpack_cache/")),
			bitsgo.NewSignResourceHandler(localResourceSigner, localResourceSigner)
	case config.AWS:
		log.Log.Infow("Creating S3 blobstore", "bucket", blobstoreConfig.S3Config.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						s3.NewBlobstoreWithLogger(*blobstoreConfig.S3Config, logger),
						metricsService,
						"buildpack_cache"),
					"buildpack_cache/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						s3.NewBlobstoreWithLogger(*blobstoreConfig.S3Config, logger),
						"buildpack_cache")),
				localResourceSigner)
	case config.Google:
		log.Log.Infow("Creating GCP blobstore", "bucket", blobstoreConfig.GCPConfig.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						gcp.NewBlobstore(*blobstoreConfig.GCPConfig),
						metricsService,
						"buildpack_cache"),
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
					decorator.ForBlobstoreWithMetricsEmitter(
						azure.NewBlobstore(*blobstoreConfig.AzureConfig, metricsService),
						metricsService,
						"buildpack_cache"),
					"buildpack_cache/")),
			bitsgo.NewSignResourceHandler(
				decorator.ForResourceSignerWithPathPartitioning(
					decorator.ForResourceSignerWithPathPrefixing(
						azure.NewBlobstore(*blobstoreConfig.AzureConfig, metricsService),
						"buildpack_cache")),
				localResourceSigner)
	case config.OpenStack:
		log.Log.Infow("Creating Openstack blobstore", "container", blobstoreConfig.OpenstackConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig),
						metricsService,
						"buildpack_cache"),
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
					decorator.ForBlobstoreWithMetricsEmitter(
						webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
						metricsService,
						"buildpack_cache"),
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
					decorator.ForBlobstoreWithMetricsEmitter(
						alibaba.NewBlobstore(*blobstoreConfig.AlibabaConfig),
						metricsService,
						"buildpack_cache"),
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

func createLocalResourceSigner(publicEndpoint *url.URL, port int, secret string, signingKeys map[string]string, activeKeyID string, resourceType string) bitsgo.ResourceSigner {
	return &local.LocalResourceSigner{
		DelegateEndpoint: fmt.Sprintf("%v://%v:%v", publicEndpoint.Scheme, publicEndpoint.Host, port),
		Signer: pathsigner.Validate(&pathsigner.PathSignerValidator{
			Secret:      secret,
			Clock:       clock.New(),
			SigningKeys: signingKeys,
			ActiveKeyID: activeKeyID,
		}),
		ResourcePathPrefix: "/" + resourceType + "/",
	}
}

func createAppStashBlobstore(blobstoreConfig config.BlobstoreConfig, publicEndpoint *url.URL, port int, secret string, signingKeys map[string]string, activeKeyID string, logger *zap.SugaredLogger, metricsService bitsgo.MetricsService) (bitsgo.Blobstore, *bitsgo.SignResourceHandler) {
	signAppStashMatchesHandler := bitsgo.NewSignResourceHandler(
		nil, // signing for get is not necessary for app_stash
		&local.LocalResourceSigner{
			DelegateEndpoint: fmt.Sprintf("%v://%v:%v", publicEndpoint.Scheme, publicEndpoint.Host, port),
			Signer: pathsigner.Validate(&pathsigner.PathSignerValidator{
				Secret:      secret,
				Clock:       clock.New(),
				SigningKeys: signingKeys,
				ActiveKeyID: activeKeyID,
			}),
			ResourcePathPrefix: "/app_stash/matches",
		})

	switch blobstoreConfig.BlobstoreType {
	case config.Local:
		log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						local.NewBlobstore(*blobstoreConfig.LocalConfig),
						metricsService,
						"app_stash"),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	case config.AWS:
		log.Log.Infow("Creating S3 blobstore", "bucket", blobstoreConfig.S3Config.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						s3.NewBlobstoreWithLogger(*blobstoreConfig.S3Config, logger),
						metricsService,
						"app_stash"),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	case config.Google:
		log.Log.Infow("Creating GCP blobstore", "bucket", blobstoreConfig.GCPConfig.Bucket)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						gcp.NewBlobstore(*blobstoreConfig.GCPConfig),
						metricsService,
						"app_stash"),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	case config.Azure:
		log.Log.Infow("Creating Azure blobstore", "container", blobstoreConfig.AzureConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						azure.NewBlobstore(*blobstoreConfig.AzureConfig, metricsService),
						metricsService,
						"app_stash"),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	case config.OpenStack:
		log.Log.Infow("Creating Openstack blobstore", "container", blobstoreConfig.OpenstackConfig.ContainerName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						openstack.NewBlobstore(*blobstoreConfig.OpenstackConfig),
						metricsService,
						"app_stash"),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	case config.WebDAV:
		log.Log.Infow("Creating Webdav blobstore",
			"public-endpoint", blobstoreConfig.WebdavConfig.PublicEndpoint,
			"private-endpoint", blobstoreConfig.WebdavConfig.PrivateEndpoint)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						webdav.NewBlobstore(*blobstoreConfig.WebdavConfig),
						metricsService,
						"app_stash"),
					blobstoreConfig.WebdavConfig.DirectoryKey+"/app_bits_cache/")),
			signAppStashMatchesHandler
	case config.Alibaba:
		log.Log.Infow("Creating Alibaba blobstore", "bucket-name", blobstoreConfig.AlibabaConfig.BucketName)
		return decorator.ForBlobstoreWithPathPartitioning(
				decorator.ForBlobstoreWithPathPrefixing(
					decorator.ForBlobstoreWithMetricsEmitter(
						alibaba.NewBlobstore(*blobstoreConfig.AlibabaConfig),
						metricsService,
						"app_stash"),
					"app_bits_cache/")),
			signAppStashMatchesHandler
	default:
		log.Log.Fatalw("blobstoreConfig is invalid.", "blobstore-type", blobstoreConfig.BlobstoreType)
		return nil, nil // satisfy compiler
	}
}

func createRootFSBlobstore(blobstoreConfig config.BlobstoreConfig) bitsgo.Blobstore {
	if blobstoreConfig.BlobstoreType != config.Local {
		log.Log.Fatalw("RootFS blobstore currently only allows local blobstores", "blobstore-type", blobstoreConfig.BlobstoreType)
	}
	log.Log.Infow("Creating local blobstore", "path-prefix", blobstoreConfig.LocalConfig.PathPrefix)
	return local.NewBlobstore(*blobstoreConfig.LocalConfig)
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

func regularlyEmitGoRoutines(metricsService bitsgo.MetricsService) {
	for range time.Tick(1 * time.Minute) {
		metricsService.SendGaugeMetric("numGoRoutines", int64(runtime.NumGoroutine()))
	}
}
