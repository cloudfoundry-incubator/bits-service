package pathsigner_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	. "github.com/benbjohnson/clock"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/cloudfoundry-incubator/bits-service/httputil"
	. "github.com/cloudfoundry-incubator/bits-service/pathsigner"
)

func TestPathSigner(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "PathSigner")
}

var _ = Describe("PathSigner", func() {

	var (
		clock  *clock.Mock
		signer *PathSignerValidator
	)

	BeforeEach(func() {
		clock = NewMock()
		signer = &PathSignerValidator{"thesecret", clock}
	})

	It("can sign a path and validate its signature", func() {
		signedPath := signer.Sign("/some/path", time.Unix(200, 0))

		Expect(signer.SignatureValid(httputil.MustParse(signedPath))).To(BeTrue())
	})

	It("can sign a path and will not validate a path when it has expired", func() {
		signedPath := signer.Sign("/some/path", time.Unix(200, 0))

		clock.Add(time.Hour)

		Expect(signer.SignatureValid(httputil.MustParse(signedPath))).To(BeFalse())
	})

	It("can sign a path and will not allow to tamper with the expiration time", func() {
		signedPath := signer.Sign("/some/path", time.Unix(200, 0))

		clock.Add(time.Hour)

		u := httputil.MustParse(signedPath)
		q := u.Query()
		q.Set("expires", fmt.Sprintf("%v", clock.Now().Add(time.Hour).Unix()))
		u.RawQuery = q.Encode()

		Expect(signer.SignatureValid(u)).To(BeFalse())
	})

})
