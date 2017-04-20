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
	Buildpacks            BlobstoreConfig
	Droplets              BlobstoreConfig
	Packages              BlobstoreConfig
	AppStash              BlobstoreConfig `yaml:"app_stash"`
	Logging               LoggingConfig
	PublicEndpoint        string `yaml:"public_endpoint"`
	PrivateEndpoint       string `yaml:"private_endpoint"`
	Secret                string
	Port                  int
	SigningUsers          []Credential `yaml:"signing_users"`
	MaxBodySize           string       `yaml:"max_body_size"`
	MetricsLogDestination string       `yaml:"metrics_log_destination"`
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

func (config *Config) MaxBodySizeBytes() uint64 {
	if config.MaxBodySize == "" {
		return 0
	}
	bytes, e := bytefmt.ToBytes(config.MaxBodySize)
	if e != nil {
		panic("Unexpected error: " + e.Error())
	}
	return bytes
}

type BlobstoreConfig struct {
	BlobstoreType string                 `yaml:"blobstore_type"`
	LocalConfig   *LocalBlobstoreConfig  `yaml:"local_config"`
	S3Config      *S3BlobstoreConfig     `yaml:"s3_config"`
	WebdavConfig  *WebdavBlobstoreConfig `yaml:"webdav_config"`
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
