package s3_blobstore

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

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

func NewS3ResourceSigner(bucket string, accessKeyID, secretAccessKey, region string) *S3ResourceSigner {
	return &S3ResourceSigner{
		s3Client: newS3Client(region, accessKeyID, secretAccessKey),
		bucket:   bucket,
	}
}

type S3ResourceSigner struct {
	s3Client *s3.S3
	bucket   string
}

func (signer *S3ResourceSigner) Sign(resource string, method string) (signedURL string) {
	var request *request.Request
	switch strings.ToLower(method) {
	case "put":
		request, _ = signer.s3Client.PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(signer.bucket),
			Key:    aws.String(pathFor(resource)),
		})
	case "get":
		request, _ = signer.s3Client.GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(signer.bucket),
			Key:    aws.String(pathFor(resource)),
		})
	default:
		panic("The only supported methods are 'put' and 'get'")
	}
	// TODO what expiration duration should we use?
	signedURL, e := request.Presign(time.Hour)
	if e != nil {
		panic(e)
	}
	log.Printf("Signed URL (verb=%v): %v", method, signedURL)
	return
}

func NewS3BuildpackCacheSigner(bucket string, accessKeyID, secretAccessKey, region string) *S3BuildpackCacheSigner {
	return &S3BuildpackCacheSigner{
		s3Client: newS3Client(region, accessKeyID, secretAccessKey),
		bucket:   bucket,
	}
}

type S3BuildpackCacheSigner struct {
	s3Client *s3.S3
	bucket   string
}

func (signer *S3BuildpackCacheSigner) Sign(resource string, method string) (signedURL string) {
	resource = strings.Replace(resource, "/entries", "", 1)
	var request *request.Request
	switch strings.ToLower(method) {
	case "put":
		request, _ = signer.s3Client.PutObjectRequest(&s3.PutObjectInput{
			Bucket: aws.String(signer.bucket),
			Key:    aws.String(resource),
		})
	case "get":
		request, _ = signer.s3Client.GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(signer.bucket),
			Key:    aws.String(resource),
		})
	default:
		panic("The only supported methods are 'put' and 'get'")
	}
	// TODO what expiration duration should we use?
	signedURL, e := request.Presign(time.Hour)
	if e != nil {
		panic(e)
	}
	log.Printf("Signed URL (verb=%v): %v", method, signedURL)
	return
}

func pathFor(identifier string) string {
	return fmt.Sprintf("/%s/%s/%s", identifier[0:2], identifier[2:4], identifier)
}
