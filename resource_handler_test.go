package bitsgo_test

import (
	"reflect"

	. "github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/httputil"

	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"io"

	. "github.com/petergtz/pegomock"
)

var _ = Describe("ResourceHandler", func() {
	Context("Put", func() {
		It("translates NoSpaceLeftError into StatusInsufficientStorage", func() {
			blobstore := NewMockBlobstore()
			handler := NewResourceHandler(blobstore, "test-resource", NewMockMetricsService(), 0)

			responseWriter := httptest.NewRecorder()
			request, e := httputil.NewPutRequest("http://notrelevant", map[string]map[string]io.Reader{"test-resource": map[string]io.Reader{"some-filename": strings.NewReader("some body")}})
			Expect(e).NotTo(HaveOccurred())

			When(blobstore.Put(AnyString(), anyReadSeeker())).ThenReturn(NewNoSpaceLeftError())
			handler.Put(responseWriter, request, map[string]string{})

			Expect(responseWriter.Code).To(Equal(http.StatusInsufficientStorage))
		})
	})
})

func anyReadSeeker() io.ReadSeeker {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*io.ReadSeeker)(nil)).Elem()))
	return nil
}
