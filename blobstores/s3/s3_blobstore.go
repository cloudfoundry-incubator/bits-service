package s3

import (
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/blobstores/s3/signer"
	"github.com/petergtz/bitsgo/blobstores/validate"
	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/logger"
	"github.com/pkg/errors"
)

type Blobstore struct {
	s3Client *s3.S3
	bucket   string
	signer   S3Signer
}

type S3Signer interface {
	Sign(req *request.Request, bucket, path string, expires time.Time) (string, error)
}

func NewBlobstore(config config.S3BlobstoreConfig) *Blobstore {
	validate.NotEmpty(config.AccessKeyID)
	validate.NotEmpty(config.Bucket)
	validate.NotEmpty(config.Region)
	validate.NotEmpty(config.SecretAccessKey)

	var s3Signer S3Signer = &signer.Default{}
	if config.Host == "storage.googleapis.com" {
		s3Signer = &signer.Google{
			AccessID:        config.AccessKeyID,
			SecretAccessKey: config.SecretAccessKey,
		}
	}

	return &Blobstore{
		s3Client: newS3Client(config.Region, config.AccessKeyID, config.SecretAccessKey, config.Host),
		bucket:   config.Bucket,
		signer:   s3Signer,
	}
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

func (blobstore *Blobstore) HeadOrRedirectAsGet(path string) (redirectLocation string, err error) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	return blobstore.signer.Sign(request, blobstore.bucket, path, time.Now().Add(time.Hour))
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	logger.Log.Debugw("Get from S3", "bucket", blobstore.bucket, "path", path)
	output, e := blobstore.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return nil, bitsgo.NewNotFoundError()
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
		Bucket: &blobstore.bucket,
		Key:    &path,
		Body:   src,
	})
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	logger.Log.Debugw("Copy in S3", "bucket", blobstore.bucket, "src", src, "dest", dest)
	_, e := blobstore.s3Client.CopyObject(&s3.CopyObjectInput{
		Key:        &dest,
		CopySource: aws.String(blobstore.bucket + "/" + src),
		Bucket:     &blobstore.bucket,
	})
	if e != nil {
		if isS3NotFoundError(e) {
			return bitsgo.NewNotFoundError()
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
			return bitsgo.NewNotFoundError()
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
			Bucket: aws.String(signer.bucket),
			Key:    aws.String(resource),
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
