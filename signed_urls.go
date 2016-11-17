package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/petergtz/bitsgo/pathsigner"
)

type SignLocalUrlHandler struct {
	Signer           *pathsigner.PathSigner
	DelegateEndpoint string
}

func (handler *SignLocalUrlHandler) Sign(responseWriter http.ResponseWriter, request *http.Request) {
	signPath := strings.Replace(request.URL.Path, "/sign", "", 1)
	fmt.Fprintf(responseWriter, "%s%s", handler.DelegateEndpoint, handler.Signer.Sign(signPath))
}

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
