package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/bits-service/oci_registry"

	"github.com/benbjohnson/clock"
	bitsgo "github.com/cloudfoundry-incubator/bits-service"
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
		ociImageHandler        *oci_registry.ImageHandler
		dropletArtifactDeleter bitsgo.DropletArtifactDeleter
		registryEndpointHost   = ""
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
		dropletArtifactDeleter = ociImageHandler.ImageManager
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
			nil,
		),
		bitsgo.NewResourceHandler(buildpackBlobstore, appStashBlobstore, "buildpack", metricsService, config.Buildpacks.MaxBodySizeBytes(), config.ShouldProxyGetRequests),
		bitsgo.NewResourceHandlerWithArtifactDeleter(
			dropletBlobstore,
			appStashBlobstore,
			"droplet",
			metricsService,
			config.Droplets.MaxBodySizeBytes(),
			config.ShouldProxyGetRequests,
			dropletArtifactDeleter),
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

func regularlyEmitGoRoutines(metricsService bitsgo.MetricsService) {
	for range time.Tick(1 * time.Minute) {
		metricsService.SendGaugeMetric("numGoRoutines", int64(runtime.NumGoroutine()))
	}
}
