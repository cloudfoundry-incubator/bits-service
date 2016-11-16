package main

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

type LocalBlobstoreConfig struct {
	PathPrefix string `yaml:"path_prefix"`
}

type S3BlobstoreConfig struct {
	Bucket         string
	AccessKey      string `yaml:"access_key"`
	SecretAccesKey string `yaml:"secret_access_key"`
}

type BlobstoreConfig struct {
	BlobstoreType string                `yaml:"blobstore_type"`
	LocalConfig   *LocalBlobstoreConfig `yaml:"local_config"`
	S3Config      *S3BlobstoreConfig    `yaml:"s3_config"`
}

type LoggingConfig struct {
	File   string
	Syslog string
	Level  string
}

type Config struct {
	Buildpacks      BlobstoreConfig
	Droplets        BlobstoreConfig
	Packages        BlobstoreConfig
	AppStash        BlobstoreConfig
	Logging         LoggingConfig
	PublicEndpoint  string `yaml:"public_endpoint"`
	PrivateEndpoint string `yaml:"private_endpoint"`
	Secret          string
	Port            int
}

func LoadConfig(filename string) (config Config, err error) {
	file, e := os.Open(filename)
	if e != nil {
		return Config{}, errors.New("error opening config. Caused by: " + e.Error())
	}
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
	}
	if config.PrivateEndpoint == "" {
		errs = append(errs, "private_endpoint must not be empty")
	}
	if len(errs) > 0 {
		return Config{}, errors.New("error in config values: " + strings.Join(errs, "; "))
	}
	return
}
