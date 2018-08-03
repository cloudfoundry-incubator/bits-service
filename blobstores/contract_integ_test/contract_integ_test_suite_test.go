package main_test

import (
	"io"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/cloudfoundry-incubator/bits-service"

	"testing"
)

func TestBlobstores(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Blobstores Contract Integration")
}

type blobstore interface {
	// Can't do the following until it is added in Go: (See also https://github.com/golang/go/issues/6977)
	// routes.Blobstore
	// routes.NoRedirectBlobstore

	// Instead doing:
	bitsgo.Blobstore
	Get(path string) (body io.ReadCloser, err error)

	bitsgo.ResourceSigner
}
