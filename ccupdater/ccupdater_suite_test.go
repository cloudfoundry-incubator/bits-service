package ccupdater_test

import (
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/petergtz/pegomock"

	"testing"
)

func TestCcUpdater(t *testing.T) {
	RegisterFailHandler(Fail)
	pegomock.RegisterMockFailHandler(ginkgo.Fail)
	RunSpecs(t, "CcUpdater Suite")
}
