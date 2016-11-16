package main

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"
	"time"

	"net/url"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type PathSigner struct {
	Secret string
}

func (signer *PathSigner) Sign(path string) string {
	return fmt.Sprintf("%s?md5=%x", path, md5.Sum([]byte(path+signer.Secret)))
}

func (signer *PathSigner) SignatureValid(u *url.URL) bool {
	return u.Query().Get("md5") == fmt.Sprintf("%x", md5.Sum([]byte(u.Path+signer.Secret)))
}

type SignLocalUrlHandler struct {
	Signer           *PathSigner
	DelegateEndpoint string
}

func (handler *SignLocalUrlHandler) Sign(responseWriter http.ResponseWriter, request *http.Request) {
	signPath := strings.Replace(request.URL.Path, "/sign", "", 1)
	fmt.Fprintf(responseWriter, "%s%s", handler.DelegateEndpoint, handler.Signer.Sign(signPath))
}

type SignatureVerificationMiddleware struct {
	Signer *PathSigner
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

type SignS3UrlHandler struct {
	s3Client *s3.S3
}

func NewSignS3UrlHandler() *SignS3UrlHandler {
	session, e := session.NewSession(&aws.Config{Region: aws.String("us-east-1")})
	if e != nil {
		panic(e)
	}
	return &SignS3UrlHandler{s3Client: s3.New(session)}
}

func (handler *SignS3UrlHandler) Sign(responseWriter http.ResponseWriter, r *http.Request) {
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
