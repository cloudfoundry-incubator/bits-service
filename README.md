Bitsgo
=======

Introduction
-------------

This is a re-implementation of the [Bits-Service](https://github.com/cloudfoundry-incubator/bits-service) in [Go](https://golang.org).

It can be used standalone or through its [BOSH-release](https://github.com/petergtz/bits-service-release).

Bitsgo passes all [system tests](https://github.com/petergtz/bits-service-release/tree/master/spec) and can therefore be used as a drop-in replacement for [Bits-Service](https://github.com/cloudfoundry-incubator/bits-service).


As blobstore backends it currently supports S3, local and WebDAV. It does *not* support additional backends through a [fog](http://fog.io/)-like library as the Ruby implementation currently does.

Getting Started
----------------

Make sure you have a working [Go environment](https://golang.org/doc/install) and the Go vendoring tool [glide]() is properly installed.

To install bitsgo:

```bash
mkdir -p $GOPATH/src/github.com/petergtz
cd $GOPATH/src/github.com/petergtz

git clone https://github.com/petergtz/bitsgo.git
cd bitsgo

glide install
go install
```

Then run it:

```
bitsgo --config my/path/to/config.yml
```
