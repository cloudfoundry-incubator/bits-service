package s3_blobstore

import (
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/petergtz/bitsgo/config"
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

func NewS3ResourceSigner(config config.S3BlobstoreConfig) *S3ResourceSigner {
	return &S3ResourceSigner{
		s3Client: newS3Client(config.Region, config.AccessKeyID, config.SecretAccessKey),
		bucket:   config.Bucket,
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
