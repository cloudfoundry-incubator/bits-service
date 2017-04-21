package middlewares_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo/middlewares"
)

var _ = Describe("MetricsMiddleWare", func() {
	It("can properly extract resource types from URL path", func() {
		Expect(middlewares.ResourceTypeFrom("/packages/123456")).To(Equal("packages"))
		Expect(middlewares.ResourceTypeFrom("/packages/123456/789")).To(Equal("packages"))
		Expect(middlewares.ResourceTypeFrom("/")).To(Equal(""))
	})
})
