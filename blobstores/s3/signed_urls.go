package s3

import (
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/petergtz/bitsgo/config"
)

func NewResourceSigner(config config.S3BlobstoreConfig) *ResourceSigner {
	return &ResourceSigner{
		s3Client: newS3Client(config.Region, config.AccessKeyID, config.SecretAccessKey),
		bucket:   config.Bucket,
	}
}

type ResourceSigner struct {
	s3Client *s3.S3
	bucket   string
}

func (signer *ResourceSigner) Sign(resource string, method string, expirationTime time.Time) (signedURL string) {
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
	// TODO use clock
	signedURL, e := request.Presign(expirationTime.Sub(time.Now()))
	if e != nil {
		panic(e)
	}
	log.Printf("Signed URL (verb=%v): %v", method, signedURL)
	return
}
