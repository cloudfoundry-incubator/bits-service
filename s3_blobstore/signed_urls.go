package s3_blobstore

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const DefaultS3Region = "us-east-1"

func newS3Client(region string, accessKeyID string, secretAccessKey string) *s3.S3 {
	session, e := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(accessKeyID, secretAccessKey, ""),
	})
	if e != nil {
		panic(e)
	}
	return s3.New(session)
}

type SignS3UrlHandler struct {
	s3Client *s3.S3
	bucket   string
}

func NewSignS3UrlHandler(bucket string, accessKeyID, secretAccessKey string) *SignS3UrlHandler {
	return &SignS3UrlHandler{
		s3Client: newS3Client(DefaultS3Region, accessKeyID, secretAccessKey),
		bucket:   bucket,
	}
}

func (handler *SignS3UrlHandler) Sign(responseWriter http.ResponseWriter, r *http.Request) {
	request, _ := handler.s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(handler.bucket),
		Key:    aws.String(strings.Replace(r.URL.String(), "/sign", "", 1)),
	})
	signedURL, e := request.Presign(5 * time.Second)
	if e != nil {
		panic(e)
	}
	fmt.Fprint(responseWriter, signedURL)
}
