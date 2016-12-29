package testutil

import (
	"io"
	"io/ioutil"

	"bytes"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
)

func HaveStatusCodeAndBody(statusCode types.GomegaMatcher, body types.GomegaMatcher) types.GomegaMatcher {
	return SatisfyAny(
		gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Code": statusCode,
			"Body": WithTransform(func(body *bytes.Buffer) string { return body.String() }, body),
		}),
		gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"StatusCode": statusCode,
			"Body": WithTransform(func(body io.Reader) string {
				content, e := ioutil.ReadAll(body)
				if e != nil {
					panic(e)
				}
				return string(content)
			}, body),
		}),
	)
}
