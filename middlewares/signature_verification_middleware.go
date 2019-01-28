package middlewares

import (
	"net/http"

	"github.com/cloudfoundry-incubator/bits-service/pathsigner"
)

type SignatureVerificationMiddleware struct {
	SignatureValidator pathsigner.PathSignatureValidator
}

func (middleware *SignatureVerificationMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	if request.URL.Query().Get("signature") == "" {
		responseWriter.WriteHeader(403)
		return
	}
	if !middleware.SignatureValidator.SignatureValid(request.Method, request.URL) {
		responseWriter.WriteHeader(403)
		return
	}
	next(responseWriter, request)
}
