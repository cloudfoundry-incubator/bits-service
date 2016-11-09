package main

import (
	"crypto/md5"
	"fmt"
	"net/http"
)

type SignatureVerifier struct {
	Secret string
}

func (l *SignatureVerifier) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if r.URL.Query().Get("md5") == "" {
		rw.WriteHeader(403)
		return
	}
	if r.URL.Query().Get("md5") != fmt.Sprintf("%x", md5.Sum([]byte(r.URL.Path+l.Secret))) {
		rw.WriteHeader(403)
		return
	}

	next(rw, r)
}
