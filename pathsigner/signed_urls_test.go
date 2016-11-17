package pathsigner_test

import (
	"testing"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo/httputil"
	. "github.com/petergtz/bitsgo/pathsigner"
)

func TestPathSigner(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "PathSigner")
}

var _ = Describe("PathSigner", func() {
	It("Can sign a path and validate its signature", func() {
		signer := &PathSigner{"thesecret"}

		signedPath := signer.Sign("/some/path")

		Expect(signer.SignatureValid(httputil.MustParse(signedPath))).To(BeTrue())
	})
})
