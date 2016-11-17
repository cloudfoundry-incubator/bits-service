package s3_blobstore_test

import (
	"testing"

	"net/http"

	"net/http/httptest"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo/httputil"
	. "github.com/petergtz/bitsgo/s3_blobstore"
)

func TestS3Blobstore(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "S3Blobstore")
}

var _ = Describe("Signing URLs", func() {

	It("Can create pre-signed URLs for S3", func() {
		signer := NewSignS3UrlHandler("mybucket", "MY-Key_ID", "dummy")
		responseWriter := httptest.NewRecorder()

		signer.Sign(responseWriter, &http.Request{URL: httputil.MustParse("/sign/my/path")})

		Expect(responseWriter.Body.String()).To(SatisfyAll(
			ContainSubstring("https://mybucket.s3.amazonaws.com/my/path"),
			ContainSubstring("X-Amz-Algorithm="),
			ContainSubstring("X-Amz-Credential=MY-Key_ID"),
			ContainSubstring("X-Amz-Date="),
			ContainSubstring("X-Amz-Expires="),
			ContainSubstring("X-Amz-Signature="),
		))
	})
})
