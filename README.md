# Bits Service
 <img src="docs/bits_logo_horizontal.svg" width="500" align="middle">


The bits-service is an extraction from existing functionality of the [cloud controller](https://github.com/cloudfoundry/cloud_controller_ng). It encapsulates all "bits operations" into its own, separately scalable service. All bits operations comprise buildpacks, droplets, app_stashes, packages and the buildpack_cache.

[The API](http://cloudfoundry-incubator.github.io/bits-service/) is a work in progress and will most likely change.

## Supported Backends

Bits currently supports [WebDAV](https://en.wikipedia.org/wiki/WebDAV) and the following [Fog](http://fog.io/) connectors:

* AWS S3
* Azure
* Google
* Local (NFS)
* Openstack


## Development

The CI config is in the [bits-service-ci](https://github.com/cloudfoundry-incubator/bits-service-ci) repo.


## Additional Notes

It can be used standalone or through its [BOSH-release](https://github.com/cloudfoundry-incubator/bits-service-release).

## Getting Started

Make sure you have a working [Go environment](https://golang.org/doc/install) and the Go vendoring tool [glide](https://github.com/Masterminds/glide#install) is properly installed.

To install bitsgo:

```bash
mkdir -p $GOPATH/src/github.com/cloudfoundry-incubator
cd $GOPATH/src/github.com/cloudfoundry-incubator

git clone https://github.com/cloudfoundry-incubator/bits-service.git
cd bits-service

glide install

cd cmd/bitsgo
go install
```

Then run it:

```
bitsgo --config my/path/to/config.yml
```

To run tests:

1. Install [ginkgo](https://onsi.github.io/ginkgo/#getting-ginkgo)
1. Configure `$PATH`:

   ```bash
   export PATH=$GOPATH/bin:$PATH
   ```

1. Run tests with

	 ```bash
	 scripts/run-unit-tests
	 ```
