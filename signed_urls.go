package main

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"
)

type SignedUrlHandler struct {
	Secret           string
	DelegateEndpoint string
}

func (handler *SignedUrlHandler) Sign(responseWriter http.ResponseWriter, request *http.Request) {
	signedPath := strings.Replace(request.URL.String(), "/sign", "", 1)
	fmt.Fprintf(responseWriter, "%s%s?md5=%x", handler.DelegateEndpoint, signedPath, md5.Sum([]byte(signedPath+handler.Secret)))
}
