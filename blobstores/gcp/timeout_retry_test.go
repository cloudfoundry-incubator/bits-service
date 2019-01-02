package gcp_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cloudfoundry-incubator/bits-service/blobstores/gcp"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
)

func TestTimeoutRetry(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Timeout retry test")
}

var _ = Describe("Timeout retry", func() {
	Context("error is context.DeadlineExceeded", func() {
		It("retries", func() {
			numRetries := 0
			e := gcp.WithRetries(2, func() error {
				e := context.DeadlineExceeded
				numRetries++
				By(fmt.Sprintf("Try #%v", numRetries))
				return gcp.TimeoutOrPermanent(e)
			})
			Expect(numRetries).To(Equal(3))
			Expect(e).To(MatchError(context.DeadlineExceeded))
		})
	})

	Context("error is any other error", func() {
		It("stops after first attempt", func() {
			numRetries := 0
			e := gcp.WithRetries(3, func() error {
				e := errors.New("Some error")
				numRetries++
				By(fmt.Sprintf("Try #%v", numRetries))
				return gcp.TimeoutOrPermanent(e)
			})
			Expect(numRetries).To(Equal(1))
			Expect(e).To(MatchError("Some error"))
		})
	})
})
