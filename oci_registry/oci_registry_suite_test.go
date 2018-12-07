package oci_registry_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOciRegistry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OciRegistry Suite")
}
