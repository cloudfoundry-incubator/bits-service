package config

import (
	"io/ioutil"
	"math"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/bytefmt"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Buildpacks BlobstoreConfig
	Droplets   BlobstoreConfig
	Packages   BlobstoreConfig
	AppStash   BlobstoreConfig `yaml:"app_stash"`

	// BuildpackCache is a Pseudo blobstore, because in reality it is using the Droplets blobstore.
	// However, we want to be able to control its max_body_size.
	BuildpackCache BlobstoreConfig `yaml:"buildpack_cache"`

	Logging         LoggingConfig
	PublicEndpoint  string `yaml:"public_endpoint"`
	PrivateEndpoint string `yaml:"private_endpoint"`
	Secret          string
	Port            int
	SigningUsers    []Credential `yaml:"signing_users"`
	MaxBodySize     string       `yaml:"max_body_size"`
	CertFile        string       `yaml:"cert_file"`
	KeyFile         string       `yaml:"key_file"`

	CCUpdater *CCUpdaterConfig `yaml:"cc_updater"`

	AppStashConfig AppStashConfig `yaml:"app_stash_config"`
}

func (config *Config) PublicEndpointUrl() *url.URL {
	u, e := url.Parse(config.PublicEndpoint)
	if e != nil {
		panic("Unexpected error: " + e.Error())
	}
	return u
}

func (config *Config) PrivateEndpointUrl() *url.URL {
	u, e := url.Parse(config.PrivateEndpoint)
	if e != nil {
		panic("Unexpected error: " + e.Error())
	}
	return u
}

type BlobstoreConfig struct {
	BlobstoreType     BlobstoreType             `yaml:"blobstore_type"`
	LocalConfig       *LocalBlobstoreConfig     `yaml:"local_config"`
	S3Config          *S3BlobstoreConfig        `yaml:"s3_config"`
	GCPConfig         *GCPBlobstoreConfig       `yaml:"gcp_config"`
	AzureConfig       *AzureBlobstoreConfig     `yaml:"azure_config"`
	OpenstackConfig   *OpenstackBlobstoreConfig `yaml:"openstack_config"`
	WebdavConfig      *WebdavBlobstoreConfig    `yaml:"webdav_config"`
	AlibabaConfig     *AlibabaBlobstoreConfig   `yaml:"alibaba_config"`
	MaxBodySize       string                    `yaml:"max_body_size"`
	GlobalMaxBodySize string                    // Not to be set by yaml
}

type BlobstoreType string

const (
	Local     BlobstoreType = "local"
	AWS       BlobstoreType = "aws"
	Google    BlobstoreType = "google"
	Azure     BlobstoreType = "azure"
	OpenStack BlobstoreType = "openstack"
	WebDAV    BlobstoreType = "webdav"
	Alibaba   BlobstoreType = "alibaba"
)

var BlobstoreTypes = map[BlobstoreType]bool{
	Local:     true,
	AWS:       true,
	Google:    true,
	Azure:     true,
	OpenStack: true,
	WebDAV:    true,
	Alibaba:   true,
}

func (config *BlobstoreConfig) MaxBodySizeBytes() uint64 {
	if config.MaxBodySize == "" {
		if config.GlobalMaxBodySize == "" {
			return 0
		}
		bytes, e := bytefmt.ToBytes(config.GlobalMaxBodySize)
		if e != nil {
			panic("Unexpected error: " + e.Error())
		}
		return bytes
	}
	bytes, e := bytefmt.ToBytes(config.MaxBodySize)
	if e != nil {
		panic("Unexpected error: " + e.Error())
	}
	return bytes
}

type LocalBlobstoreConfig struct {
	PathPrefix string `yaml:"path_prefix"`
}

type S3BlobstoreConfig struct {
	Bucket          string
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	Region          string
	Host            string `yaml:",omitempty"`
}

type GCPBlobstoreConfig struct {
	Bucket       string
	PrivateKeyID string `yaml:"private_key_id"`
	PrivateKey   string `yaml:"private_key"`
	Email        string
	TokenURL     string `yaml:"token_url"`
}

type AzureBlobstoreConfig struct {
	ContainerName string `yaml:"container_name"`
	AccountName   string `yaml:"account_name"`
	AccountKey    string `yaml:"account_key"`
	Environment   string
}

func (c *AzureBlobstoreConfig) EnvironmentName() string {
	if c.Environment != "" {
		return c.Environment
	}
	return "AzurePublicCloud"
}

type OpenstackBlobstoreConfig struct {
	ContainerName  string `yaml:"container_name"`
	DomainName     string `yaml:"domain_name"`
	DomainId       string `yaml:"domain_id"`
	Username       string
	ApiKey         string `yaml:"api_key"`
	AuthURL        string `yaml:"auth_url"`
	Region         string
	AuthVersion    int    `yaml:"auth_version"`
	Internal       bool   // Set this to true to use the the internal / service network
	Tenant         string // Name of the tenant (v2,v3 auth only)
	TenantId       string `yaml:"tenant_id"`        // Id of the tenant (v2,v3 auth only)
	EndpointType   string `yaml:"endpoint_type"`    // Endpoint type (v2,v3 auth only) (default is public URL unless Internal is set)
	TenantDomain   string `yaml:"tenant_domain"`    // Name of the tenant's domain (v3 auth only), only needed if it differs from the user domain
	TenantDomainId string `yaml:"tenant_domain_id"` // Id of the tenant's domain (v3 auth only), only needed if it differs the from user domain
	TrustId        string `yaml:"trust_id"`         // Id of the trust (v3 auth only)

	AccountMetaTempURLKey string `yaml:"account_meta_temp_url_key"` // used as secret for signed URLs
}

type WebdavBlobstoreConfig struct {
	DirectoryKey    string `yaml:"directory_key"`
	PrivateEndpoint string `yaml:"private_endpoint"`
	PublicEndpoint  string `yaml:"public_endpoint"`
	CACertPath      string `yaml:"ca_cert_path"`
	SkipCertVerify  bool   `yaml:"skip_cert_verify"`
	Username        string
	Password        string
}

type AlibabaBlobstoreConfig struct {
	BucketName string `yaml:"bucket_name"`
	ApiKey     string `yaml:"access_key_id"`
	ApiSecret  string `yaml:"access_key_secret"`
	Endpoint   string
}

func (config WebdavBlobstoreConfig) CACert() string {
	caCert, e := ioutil.ReadFile(config.CACertPath)
	if e != nil {
		panic(errors.Wrapf(e, "Error while reading CA cert file \"%v\"", config.CACertPath))
	}
	return string(caCert)
}

type Credential struct {
	Username string
	Password string
}

type LoggingConfig struct {
	Level string
}

type CCUpdaterConfig struct {
	Endpoint       string
	Method         string
	ClientCertFile string `yaml:"client_cert_file"`
	ClientKeyFile  string `yaml:"client_key_file"`
	CACertFile     string `yaml:"ca_cert_file"`
}

type AppStashConfig struct {
	MinimumSize string `yaml:"minimum_size"`
	MaximumSize string `yaml:"maximum_size"`
}

func (config *AppStashConfig) MinimumSizeBytes() uint64 {
	return parseSizeProperty(config.MinimumSize, 0)
}
func (config *AppStashConfig) MaximumSizeBytes() uint64 {
	return parseSizeProperty(config.MaximumSize, math.MaxUint64)
}

func parseSizeProperty(size string, defaultValue uint64) uint64 {
	if size == "" {
		return defaultValue
	}
	bytes, e := bytefmt.ToBytes(size)
	if e != nil {
		panic("Unexpected error: " + e.Error())
	}
	return bytes
}

func LoadConfig(filename string) (config Config, err error) {
	file, e := os.Open(filename)
	if e != nil {
		return Config{}, errors.New("error opening config. Caused by: " + e.Error())
	}
	defer file.Close()
	content, e := ioutil.ReadAll(file)
	if e != nil {
		return Config{}, errors.New("error reading config. Caused by: " + e.Error())
	}
	e = yaml.Unmarshal(content, &config)
	if e != nil {
		return Config{}, errors.New("error parsing config. Caused by: " + e.Error())
	}
	config.Droplets.GlobalMaxBodySize = config.MaxBodySize
	config.Packages.GlobalMaxBodySize = config.MaxBodySize
	config.AppStash.GlobalMaxBodySize = config.MaxBodySize
	config.Buildpacks.GlobalMaxBodySize = config.MaxBodySize
	config.BuildpackCache.GlobalMaxBodySize = config.MaxBodySize

	config.Droplets.BlobstoreType = BlobstoreType(strings.ToLower(string(config.Droplets.BlobstoreType)))
	config.Packages.BlobstoreType = BlobstoreType(strings.ToLower(string(config.Packages.BlobstoreType)))
	config.AppStash.BlobstoreType = BlobstoreType(strings.ToLower(string(config.AppStash.BlobstoreType)))
	config.Buildpacks.BlobstoreType = BlobstoreType(strings.ToLower(string(config.Buildpacks.BlobstoreType)))

	var errs []string

	if config.Port == 0 {
		errs = append(errs, "port must be an integer > 0")
	}
	if config.PublicEndpoint == "" {
		errs = append(errs, "public_endpoint must not be empty")
	} else {
		publicEndpoint, e := url.Parse(config.PublicEndpoint)
		if e != nil {
			errs = append(errs, "public_endpoint is invalid. Caused by:"+e.Error())
		} else {
			if publicEndpoint.Host == "" {
				errs = append(errs, "public_endpoint host must not be empty")
			}
			if publicEndpoint.Scheme != "https" {
				errs = append(errs, "public_endpoint must use https://")
			}
		}
	}
	if config.PrivateEndpoint == "" {
		errs = append(errs, "private_endpoint must not be empty")
	} else {
		privateEndpoint, e := url.Parse(config.PrivateEndpoint)
		if e != nil {
			errs = append(errs, "private_endpoint is invalid. Caused by:"+e.Error())
		} else {
			if privateEndpoint.Host == "" {
				errs = append(errs, "private_endpoint host must not be empty")
			}
			if privateEndpoint.Scheme != "https" {
				errs = append(errs, "private_endpoint must use https://")
			}
		}
	}
	if config.CertFile == "" {
		errs = append(errs, "cert_file must not be empty")
	}
	if config.KeyFile == "" {
		errs = append(errs, "key_file must not be empty")
	}
	if config.MaxBodySize != "" {
		_, e = bytefmt.ToBytes(config.MaxBodySize)
		if e != nil {
			errs = append(errs, "max_body_size is invalid. Caused by: "+e.Error())
		}
	}

	if config.CCUpdater != nil {
		config.CCUpdater.Method = "PATCH"
		u, e := url.Parse(config.CCUpdater.Endpoint)
		if e != nil {
			errs = append(errs, "cc_updater.endpoint is invalid. Caused by:"+e.Error())
		} else if u.Host == "" {
			errs = append(errs, "cc_updater.endpoint host must not be empty")
		}
	}

	verifyBlobstoreType(config.Droplets.BlobstoreType, "droplets", &errs)
	verifyBlobstoreType(config.Packages.BlobstoreType, "packages", &errs)
	verifyBlobstoreType(config.AppStash.BlobstoreType, "app_stash", &errs)
	verifyBlobstoreType(config.Buildpacks.BlobstoreType, "buildpacks", &errs)

	verifyBlobstoreConfig(config.Droplets, "droplets", &errs)
	verifyBlobstoreConfig(config.Packages, "packages", &errs)
	verifyBlobstoreConfig(config.Buildpacks, "buildpacks", &errs)
	verifyBlobstoreConfig(config.AppStash, "app_stash", &errs)

	if len(errs) > 0 {
		// returning here already, because follow-up checks are difficult if not even basic checks succeed
		return Config{}, errors.New("error in config values: " + strings.Join(errs, "; "))
	}

	if config.BuildpackCache.AzureConfig != nil ||
		config.BuildpackCache.GCPConfig != nil ||
		config.BuildpackCache.LocalConfig != nil ||
		config.BuildpackCache.OpenstackConfig != nil ||
		config.BuildpackCache.S3Config != nil ||
		config.BuildpackCache.AlibabaConfig != nil ||
		config.BuildpackCache.WebdavConfig != nil {
		errs = append(errs, "buildpack_cache must not have a blobstore configured, as it only exists to allow to configure max_body_size. "+
			"As blobstore, the droplet blobstore is used.")
	}

	if config.Packages.BlobstoreType == WebDAV && config.Packages.WebdavConfig.DirectoryKey == "" {
		errs = append(errs, "Packages WebDAV blobstore must have a directory_key configured.")
	}
	if config.Droplets.BlobstoreType == WebDAV && config.Droplets.WebdavConfig.DirectoryKey == "" {
		errs = append(errs, "Droplets WebDAV blobstore must have a directory_key configured.")
	}
	if config.Buildpacks.BlobstoreType == WebDAV && config.Buildpacks.WebdavConfig.DirectoryKey == "" {
		errs = append(errs, "Buildpacks WebDAV blobstore must have a directory_key configured.")
	}
	if config.AppStash.BlobstoreType == WebDAV && config.AppStash.WebdavConfig.DirectoryKey == "" {
		errs = append(errs, "AppStash WebDAV blobstore must have a directory_key configured.")
	}

	if config.AppStashConfig.MinimumSizeBytes() > config.AppStashConfig.MaximumSizeBytes() {
		errs = append(errs, "app_stash_config.maximum_size must be greater than app_stash_config.minimum_size")
	}

	// TODO validate CACertsPaths
	if len(errs) > 0 {
		return Config{}, errors.New("error in config values: " + strings.Join(errs, "; "))
	}
	return
}

func verifyBlobstoreType(blobstoreType BlobstoreType, resourceType string, errs *[]string) {
	if !BlobstoreTypes[blobstoreType] {
		blobstoreKeys := make([]string, 0)
		for key := range BlobstoreTypes {
			blobstoreKeys = append(blobstoreKeys, string(key))
		}
		*errs = append(*errs, "Blobstore type '"+string(blobstoreType)+"' for "+resourceType+" is invalid. Valid blobstore types are: "+strings.Join(blobstoreKeys, ", "))
	}
}

func verifyBlobstoreConfig(blobstoreConfig BlobstoreConfig, resourceType string, errs *[]string) {
	if blobstoreConfig.BlobstoreType == "" { // Already handled
		return
	}
	if blobstoreConfigIsNil(blobstoreConfig) {
		*errs = append(*errs, resourceType+" blobstore config is missing "+string(blobstoreConfig.BlobstoreType)+" config")
	}
}

func blobstoreConfigIsNil(blobstoreConfig BlobstoreConfig) bool {
	switch blobstoreConfig.BlobstoreType {
	case AWS:
		return blobstoreConfig.S3Config == nil || *blobstoreConfig.S3Config == (S3BlobstoreConfig{})
	case Local:
		return blobstoreConfig.LocalConfig == nil || *blobstoreConfig.LocalConfig == (LocalBlobstoreConfig{})
	case OpenStack:
		return blobstoreConfig.OpenstackConfig == nil || *blobstoreConfig.OpenstackConfig == (OpenstackBlobstoreConfig{})
	case Azure:
		return blobstoreConfig.AzureConfig == nil || *blobstoreConfig.AzureConfig == (AzureBlobstoreConfig{})
	case Google:
		return blobstoreConfig.GCPConfig == nil || *blobstoreConfig.GCPConfig == (GCPBlobstoreConfig{})
	case WebDAV:
		return blobstoreConfig.WebdavConfig == nil || *blobstoreConfig.WebdavConfig == (WebdavBlobstoreConfig{})
	default:
		return true
	}
}
