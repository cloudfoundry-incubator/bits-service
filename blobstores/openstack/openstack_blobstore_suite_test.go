package openstack_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOpenstack(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Openstack Suite")
}
