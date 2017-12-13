package config

import (
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/bytefmt"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Buildpacks      BlobstoreConfig
	Droplets        BlobstoreConfig
	Packages        BlobstoreConfig
	AppStash        BlobstoreConfig `yaml:"app_stash"`
	Logging         LoggingConfig
	PublicEndpoint  string `yaml:"public_endpoint"`
	PrivateEndpoint string `yaml:"private_endpoint"`
	Secret          string
	Port            int
	SigningUsers    []Credential `yaml:"signing_users"`
	MaxBodySize     string       `yaml:"max_body_size"`
	CertFile        string       `yaml:"cert_file"`
	KeyFile         string       `yaml:"key_file"`
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
	BlobstoreType     string                    `yaml:"blobstore_type"`
	LocalConfig       *LocalBlobstoreConfig     `yaml:"local_config"`
	S3Config          *S3BlobstoreConfig        `yaml:"s3_config"`
	GCPConfig         *GCPBlobstoreConfig       `yaml:"gcp_config"`
	AzureConfig       *AzureBlobstoreConfig     `yaml:"azure_config"`
	OpenstackConfig   *OpenstackBlobstoreConfig `yaml:"openstack_config"`
	WebdavConfig      *WebdavBlobstoreConfig    `yaml:"webdav_config"`
	MaxBodySize       string                    `yaml:"max_body_size"`
	GlobalMaxBodySize string                    // Not to be set by yaml
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
	PrivateEndpoint string `yaml:"private_endpoint"`
	PublicEndpoint  string `yaml:"public_endpoint"`
	CACertPath      string `yaml:"ca_cert_path"`
	SkipCertVerify  bool   `yaml:"skip_cert_verify"`
	Username        string
	Password        string
}

func (config WebdavBlobstoreConfig) CACert() string {
	caCert, e := ioutil.ReadFile(config.CACertPath)
	if e != nil {
		panic(e)
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
		} else if publicEndpoint.Host == "" {
			errs = append(errs, "public_endpoint host must not be empty")
		}
	}
	if config.PrivateEndpoint == "" {
		errs = append(errs, "private_endpoint must not be empty")
	} else {
		privateEndpoint, e := url.Parse(config.PrivateEndpoint)
		if e != nil {
			errs = append(errs, "private_endpoint is invalid. Caused by:"+e.Error())
		} else if privateEndpoint.Host == "" {
			errs = append(errs, "private_endpoint host must not be empty")
		}
	}
	if config.MaxBodySize != "" {
		_, e = bytefmt.ToBytes(config.MaxBodySize)
		if e != nil {
			errs = append(errs, "max_body_size is invalid. Caused by: "+e.Error())
		}
	}
	// TODO validate CACertsPaths
	if len(errs) > 0 {
		return Config{}, errors.New("error in config values: " + strings.Join(errs, "; "))
	}
	return
}
