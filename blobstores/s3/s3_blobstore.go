package s3

import (
	"fmt"
	"io"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/cloudfoundry-incubator/bits-service"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/s3/signer"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/validate"
	"github.com/cloudfoundry-incubator/bits-service/config"
	"github.com/cloudfoundry-incubator/bits-service/logger"
	log "github.com/cloudfoundry-incubator/bits-service/logger"
	"github.com/pkg/errors"
)

type Blobstore struct {
	s3Client             *s3.S3
	bucket               string
	signer               S3Signer
	serverSideEncryption *string
	sseKMSKeyID          *string
}

type S3Signer interface {
	Sign(req *request.Request, bucket, path string, expires time.Time) (string, error)
}

const (
	AES256 = "AES256"
	AWSKMS = "aws:kms"
)

func NewBlobstore(config config.S3BlobstoreConfig) *Blobstore {
	return NewBlobstoreWithLogger(config, log.Log)
}

func NewBlobstoreWithLogger(config config.S3BlobstoreConfig, logger *zap.SugaredLogger) *Blobstore {
	if !config.UseIAMProfile {
		validate.NotEmpty(config.AccessKeyID)
		validate.NotEmpty(config.SecretAccessKey)
	}
	validate.NotEmpty(config.Bucket)

	var s3Signer S3Signer = &signer.Default{}
	if config.Host == "storage.googleapis.com" {
		s3Signer = &signer.Google{
			AccessID:        config.AccessKeyID,
			SecretAccessKey: config.SecretAccessKey,
		}
	}

	// Make SDK happy. The AWS Client requires a region, although it never uses that region,
	// because S3 is region independent. (It's using that region when used with other AWS services.)
	if config.Region == "" {
		config.Region = "us-east-1"
		log.Log.Infow("No AWS region specified for blobstore. Using a default value.", "bucket", config.Bucket, "default-region", config.Region)
	}

	if config.SignatureVersion == 2 {
		if config.UseIAMProfile {
			logger.Fatalw("Blobstore is configured to use EC2 instance roles (use-iam-profiles) and also to use S3 signature version 2 " +
				"(https://docs.aws.amazon.com/AmazonS3/latest/dev/RESTAuthentication.html). But EC2 instance roles are only supported with " +
				"S3 signature version 4.")
		}
		if config.ServerSideEncryption != "" {
			logger.Fatalw("Blobstore is configured to use server side encryption and also to use S3 signature version 2 " +
				"(https://docs.aws.amazon.com/AmazonS3/latest/dev/RESTAuthentication.html). But server side encryption is only supported with " +
				"S3 signature version 4.")
		}
	}

	blobstore := &Blobstore{
		s3Client: newS3Client(config.Region,
			config.UseIAMProfile,
			config.AccessKeyID,
			config.SecretAccessKey,
			config.Host,
			logger,
			config.S3DebugLogLevel,
			config.Bucket,
			config.SignatureVersion,
		),
		bucket: config.Bucket,
		signer: s3Signer,
	}

	if config.ServerSideEncryption != "" {
		if config.ServerSideEncryption != AES256 &&
			config.ServerSideEncryption != AWSKMS {
			panic(fmt.Errorf("Server Side Encryption value invalid (%v). Must be either empty, AES256, or aws:kms", config.ServerSideEncryption))
		}
		blobstore.serverSideEncryption = aws.String(config.ServerSideEncryption)
		if config.ServerSideEncryption == AWSKMS {
			validate.NotEmpty(config.SSEKMSKeyID)
			blobstore.sseKMSKeyID = aws.String(config.SSEKMSKeyID)
		}
	}

	return blobstore
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	_, e := blobstore.s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return false, nil
		}
		return false, errors.Wrapf(e, "Failed to check for %v/%v", blobstore.bucket, path)
	}
	return true, nil
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	logger.Log.Debugw("Get from S3", "bucket", blobstore.bucket, "path", path)
	output, e := blobstore.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return nil, bitsgo.NewNotFoundErrorWithKey(path)
		}
		return nil, errors.Wrapf(e, "Path %v", path)
	}
	return output.Body, nil
}

func (blobstore *Blobstore) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedUrl, e := blobstore.signer.Sign(request, blobstore.bucket, path, time.Now().Add(time.Hour))
	return nil, signedUrl, e
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	logger.Log.Debugw("Put to S3", "bucket", blobstore.bucket, "path", path)
	_, e := blobstore.s3Client.PutObject(&s3.PutObjectInput{
		Bucket:               &blobstore.bucket,
		Key:                  &path,
		Body:                 src,
		ServerSideEncryption: blobstore.serverSideEncryption,
		SSEKMSKeyId:          blobstore.sseKMSKeyID,
	})
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	// see https://forums.aws.amazon.com/thread.jspa?threadID=55746:
	src = strings.Replace(src, "+", "%2B", -1)

	logger.Log.Debugw("Copy in S3", "bucket", blobstore.bucket, "src", src, "dest", dest)
	_, e := blobstore.s3Client.CopyObject(&s3.CopyObjectInput{
		Key:                  &dest,
		CopySource:           aws.String(blobstore.bucket + "/" + src),
		Bucket:               &blobstore.bucket,
		ServerSideEncryption: blobstore.serverSideEncryption,
		SSEKMSKeyId:          blobstore.sseKMSKeyID,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return bitsgo.NewNotFoundErrorWithKey(src)
		}
		return errors.Wrapf(e, "Error while trying to copy src %v to dest %v in bucket %v", src, dest, blobstore.bucket)
	}
	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	_, e := blobstore.s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return bitsgo.NewNotFoundErrorWithKey(path)
		}
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	deletionErrs := []error{}
	e := blobstore.s3Client.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket: &blobstore.bucket,
			Prefix: &prefix,
		},
		func(p *s3.ListObjectsOutput, lastPage bool) (shouldContinue bool) {
			for _, object := range p.Contents {
				e := blobstore.Delete(*object.Key)
				if e != nil {
					if _, isNotFoundError := e.(*bitsgo.NotFoundError); !isNotFoundError {
						deletionErrs = append(deletionErrs, e)
					}
				}
			}
			return true
		})
	if e != nil {
		return errors.Wrapf(e, "Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	if len(deletionErrs) != 0 {
		return errors.Errorf("Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	return nil
}

func (signer *Blobstore) Sign(resource string, method string, expirationTime time.Time) (signedURL string) {
	var request *request.Request
	switch strings.ToLower(method) {
	case "put":
		request, _ = signer.s3Client.PutObjectRequest(&s3.PutObjectInput{
			Bucket:               aws.String(signer.bucket),
			Key:                  aws.String(resource),
			ServerSideEncryption: signer.serverSideEncryption,
			SSEKMSKeyId:          signer.sseKMSKeyID,
		})
	case "get":
		request, _ = signer.s3Client.GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(signer.bucket),
			Key:    aws.String(resource),
		})
	default:
		panic("The only supported methods are 'put' and 'get'. But got '" + method + "'")
	}
	// TODO use clock
	signedURL, e := signer.signer.Sign(request, signer.bucket, resource, expirationTime)
	if e != nil {
		panic(e)
	}
	logger.Log.Debugw("Signed URL", "verb", method, "signed-url", signedURL)
	return
}
