package s3

import (
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/logger"
	"github.com/pkg/errors"
)

type Blobstore struct {
	s3Client *s3.S3
	bucket   string
}

func NewBlobstore(config config.S3BlobstoreConfig) *Blobstore {
	return &Blobstore{
		s3Client: newS3Client(config.Region, config.AccessKeyID, config.SecretAccessKey),
		bucket:   config.Bucket,
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
	return signedURLFrom(request, blobstore.bucket, path)
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
	signedUrl, e := signedURLFrom(request, blobstore.bucket, path)
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

func (blobstore *Blobstore) PutOrRedirect(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	// This is the behavior as in the current Ruby implementation
	e := blobstore.Put(path, src)
	return "", e

	// Could also think of a redirect implementation:

	// request, _ := blobstore.s3Client.PutObjectRequest(&s3.PutObjectInput{
	// 	Bucket: &blobstore.bucket,
	// 	Key:    &path,
	// })
	// return signedURLFrom(request, blobstore.bucket, path)
}

func signedURLFrom(req *request.Request, bucket, path string) (string, error) {
	signedURL, e := req.Presign(time.Hour)
	if e != nil {
		return "", errors.Wrapf(e, "Bucket/Path %v/%v", bucket, path)
	}
	return signedURL, nil

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
