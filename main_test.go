package main_test

import (
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/petergtz/pegomock"
)

func TestMain(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	pegomock.RegisterMockFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Main")
}
