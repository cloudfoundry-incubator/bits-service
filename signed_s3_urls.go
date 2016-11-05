package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type SignedS3UrlHandler struct {
	s3Client *s3.S3
}

func NewSignedS3UrlHandler() *SignedS3UrlHandler {
	session, e := session.NewSession(&aws.Config{Region: aws.String("us-east-1")})
	if e != nil {
		panic(e)
	}
	return &SignedS3UrlHandler{s3Client: s3.New(session)}
}

func (handler *SignedS3UrlHandler) Sign(responseWriter http.ResponseWriter, r *http.Request) {
	request, _ := handler.s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String("mybucket"),
		Key:    aws.String(strings.Replace(r.URL.String(), "/sign", "", 1)),
	})
	signedURL, e := request.Presign(5 * time.Second)
	if e != nil {
		panic(e)
	}
	fmt.Fprint(responseWriter, signedURL)
}
