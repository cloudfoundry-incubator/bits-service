package main_test

import (
	"github.com/cloudfoundry-incubator/bits-service"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	"testing"
)

func TestBlobstores(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Blobstores Contract Integration")
}

type blobstore interface {
	bitsgo.Blobstore
	bitsgo.ResourceSigner
}
