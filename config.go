package main

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

type BlostoreConfig struct {
}

type LoggingConfig struct {
	File   string
	Syslog string
	Level  string
}

type Config struct {
	Buildpacks      BlostoreConfig
	Droplets        BlostoreConfig
	Packages        BlostoreConfig
	AppStash        BlostoreConfig
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
