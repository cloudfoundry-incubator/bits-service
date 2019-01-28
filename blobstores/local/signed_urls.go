package local

import (
	"fmt"

	"time"

	"github.com/cloudfoundry-incubator/bits-service/pathsigner"
)

type LocalResourceSigner struct {
	Signer             pathsigner.PathSigner
	ResourcePathPrefix string
	DelegateEndpoint   string
}

func (signer *LocalResourceSigner) Sign(resource string, method string, expirationTime time.Time) (signedURL string) {
	return fmt.Sprintf("%s%s", signer.DelegateEndpoint, signer.Signer.Sign(method, signer.ResourcePathPrefix+resource, expirationTime))
}
