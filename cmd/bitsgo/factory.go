package main

import (
	"fmt"
	"net/url"

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
	"github.com/cloudfoundry-incubator/bits-service/ccupdater"
	"github.com/cloudfoundry-incubator/bits-service/config"
	log "github.com/cloudfoundry-incubator/bits-service/logger"
	"github.com/cloudfoundry-incubator/bits-service/pathsigner"
	"go.uber.org/zap"
)

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
