package main

import (
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
)

type S3PureRedirectBlobStore struct {
	s3Client *s3.S3
	bucket   string
}

func (blobstore *S3PureRedirectBlobStore) Get(path string, responseWriter http.ResponseWriter) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(5 * time.Second)
	if e != nil {
		panic(e)
	}
	http.Redirect(responseWriter, nil, signedURL, 302)
}

func (blobstore *S3PureRedirectBlobStore) Put(path string, src io.Reader, responseWriter http.ResponseWriter) {
	request, _ := blobstore.s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(5 * time.Second)
	if e != nil {
		panic(e)
	}
	http.Redirect(responseWriter, nil, signedURL, 302)
}
