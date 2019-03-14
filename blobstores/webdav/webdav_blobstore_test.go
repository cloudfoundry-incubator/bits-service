package webdav_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/bits-service/blobstores/webdav"
)

var _ = Describe("WebdavBlobstore", func() {

	It("appends a slash if there is none", func() {
		prefix := AppendsSuffixIfNeeded("/some/path")
		Expect(prefix).ToNot(HaveSuffix("//"))
		Expect(prefix).To(Equal("/some/path/"))

	})
	It("does not append a slash if there already is a slash at the end", func() {
		prefix := AppendsSuffixIfNeeded("/some/path/")
		Expect(prefix).ToNot(HaveSuffix("//"))
		Expect(prefix).To(Equal("/some/path/"))
	})

})
