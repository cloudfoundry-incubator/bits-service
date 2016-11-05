package main

import (
	"io"
	"net/http"
)

type S3PureRedirectBlobStore struct {
}

func (blobstore *S3PureRedirectBlobStore) Get(path string, responseWriter http.ResponseWriter) {
	http.Redirect(responseWriter, nil, blobstore.generatePreSignedS3Url(path), 302)
}

func (blobstore *S3PureRedirectBlobStore) Put(path string, src io.Reader, responseWriter http.ResponseWriter) {
	http.Redirect(responseWriter, nil, blobstore.generatePreSignedS3Url(path), 302)
}

func (blobstore *S3PureRedirectBlobStore) generatePreSignedS3Url(path string) string {
	return "http://pre-signed-s3-url"
}
