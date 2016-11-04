package main

import (
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type SignedUrlHandler struct {
	Secret           string
	Delegate         http.Handler
	DelegateEndpoint string
}

func (handler *SignedUrlHandler) Sign(responseWriter http.ResponseWriter, request *http.Request) {
	signedPath := "/signed" + strings.Replace(request.URL.String(), "/sign", "", 1)
	fmt.Fprintf(responseWriter, "%s%s?md5=%x", handler.DelegateEndpoint, signedPath, md5.Sum([]byte(signedPath+handler.Secret)))
}

func (handler *SignedUrlHandler) Decode(responseWriter http.ResponseWriter, request *http.Request) {
	if !handler.validSignature(request) {
		responseWriter.WriteHeader(403)
		return
	}
	r := *request
	url, e := url.ParseRequestURI(strings.Replace(request.URL.String(), "/signed", "", 1))
	if e != nil {
		log.Fatal(e)
	}
	r.URL = url
	r.RequestURI = url.RequestURI()
	handler.Delegate.ServeHTTP(responseWriter, &r)
}

func (handler *SignedUrlHandler) validSignature(request *http.Request) bool {
	return request.URL.Query().Get("md5") == fmt.Sprintf("%x", md5.Sum([]byte(request.URL.Path+handler.Secret)))
}
