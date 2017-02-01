package pathsigner_test

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
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
		clock := clock.NewMock()

		signer := &PathSignerValidator{"thesecret", clock}

		signedPath := signer.Sign("/some/path", time.Unix(200, 0))

		Expect(signer.SignatureValid(httputil.MustParse(signedPath))).To(BeTrue())
	})
})
