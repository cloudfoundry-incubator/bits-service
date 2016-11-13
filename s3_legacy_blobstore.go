package main

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
)

type S3LegacyBlobStore struct {
	s3Client *s3.S3
	bucket   string
}

func (blobstore *S3LegacyBlobStore) Get(path string, responseWriter http.ResponseWriter) {
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

func (blobstore *S3LegacyBlobStore) Put(path string, src io.ReadSeeker, responseWriter http.ResponseWriter) {
	_, e := blobstore.s3Client.PutObject(&s3.PutObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
		Body:   src,
	})
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	responseWriter.WriteHeader(201)
}
