package webdav_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestWebdav(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webdav Suite")
}
