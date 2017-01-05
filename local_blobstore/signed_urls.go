package local_blobstore

import (
	"fmt"
	"net/http"

	"github.com/petergtz/bitsgo/pathsigner"
)

type SignatureVerificationMiddleware struct {
	Signer *pathsigner.PathSigner
}

func (middleware *SignatureVerificationMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	if request.URL.Query().Get("md5") == "" {
		responseWriter.WriteHeader(403)
		return
	}
	if !middleware.Signer.SignatureValid(request.URL) {
		responseWriter.WriteHeader(403)
		return
	}
	next(responseWriter, request)
}

type LocalResourceSigner struct {
	Signer             *pathsigner.PathSigner
	ResourcePathPrefix string
	DelegateEndpoint   string
}

func (signer *LocalResourceSigner) Sign(resource string, method string) (signedURL string) {
	return fmt.Sprintf("%s%s", signer.DelegateEndpoint, signer.Signer.Sign(signer.ResourcePathPrefix+resource))
}
