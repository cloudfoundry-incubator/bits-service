package s3_blobstore

import (
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/routes"
	"github.com/pkg/errors"
)

type S3LegacyBlobStore struct {
	pureRedirect *S3PureRedirectBlobStore
	noRedirect   *S3NoRedirectBlobStore
}

func NewS3LegacyBlobstore(config config.S3BlobstoreConfig) *S3LegacyBlobStore {
	return &S3LegacyBlobStore{
		pureRedirect: NewS3PureRedirectBlobstore(config),
		noRedirect:   NewS3NoRedirectBlobStore(config),
	}
}

func (blobstore *S3LegacyBlobStore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	return blobstore.pureRedirect.Get(path)
}

func (blobstore *S3LegacyBlobStore) Head(path string) (redirectLocation string, err error) {
	return blobstore.pureRedirect.Head(path)
}

func (blobstore *S3LegacyBlobStore) Put(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	return blobstore.noRedirect.Put(path, src)
}

func (blobstore *S3LegacyBlobStore) Exists(path string) (bool, error) {
	return blobstore.noRedirect.Exists(path)
}

func (blobstore *S3LegacyBlobStore) Delete(path string) error {
	return blobstore.noRedirect.Delete(path)
}

type S3PureRedirectBlobStore struct {
	s3Client *s3.S3
	bucket   string
}

func NewS3PureRedirectBlobstore(config config.S3BlobstoreConfig) *S3PureRedirectBlobStore {
	return &S3PureRedirectBlobStore{
		s3Client: newS3Client(config.Region, config.AccessKeyID, config.SecretAccessKey),
		bucket:   config.Bucket,
	}
}

func (blobstore *S3PureRedirectBlobStore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedUrl, e := signedURLFrom(request, blobstore.bucket, path)
	return nil, signedUrl, e
}

func (blobstore *S3PureRedirectBlobStore) Head(path string) (redirectLocation string, err error) {
	request, _ := blobstore.s3Client.HeadObjectRequest(&s3.HeadObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	return signedURLFrom(request, blobstore.bucket, path)
}

func (blobstore *S3PureRedirectBlobStore) Put(path string, src io.Reader) (redirectLocation string, err error) {
	request, _ := blobstore.s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	return signedURLFrom(request, blobstore.bucket, path)
}

func signedURLFrom(req *request.Request, bucket, path string) (string, error) {
	signedURL, e := req.Presign(time.Hour)
	if e != nil {
		return "", errors.Wrapf(e, "Bucket/Path %v/%v", bucket, path)
	}
	return signedURL, nil

}

func (blobstore *S3PureRedirectBlobStore) Delete(path string) error {
	_, e := blobstore.s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}

type S3NoRedirectBlobStore struct {
	s3Client *s3.S3
	bucket   string
}

func NewS3NoRedirectBlobStore(config config.S3BlobstoreConfig) *S3NoRedirectBlobStore {
	return &S3NoRedirectBlobStore{
		s3Client: newS3Client(config.Region, config.AccessKeyID, config.SecretAccessKey),
		bucket:   config.Bucket,
	}
}

func (blobstore *S3NoRedirectBlobStore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	output, e := blobstore.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if ae, isAwsErr := e.(awserr.Error); isAwsErr {
			if ae.Code() == "NoSuchBucket" || ae.Code() == "NoSuchKey" || ae.Code() == "NotFound" {
				return nil, "", routes.NewNotFoundError()
			}
		}
		return nil, "", errors.Wrapf(e, "Path %v", path)
	}
	return output.Body, "", nil
}

func (blobstore *S3NoRedirectBlobStore) Head(path string) (redirectLocation string, err error) {
	_, e := blobstore.s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if ae, isAwsErr := e.(awserr.Error); isAwsErr {
			if ae.Code() == "NoSuchBucket" || ae.Code() == "NoSuchKey" || ae.Code() == "NotFound" {
				return "", routes.NewNotFoundError()
			}
		}
		return "", errors.Wrapf(e, "Path %v", path)
	}
	return "", nil
}

func (blobstore *S3NoRedirectBlobStore) Put(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	_, e := blobstore.s3Client.PutObject(&s3.PutObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
		Body:   src,
	})
	if e != nil {
		return "", errors.Wrapf(e, "Path %v", path)
	}
	return "", nil
}

func (blobstore *S3NoRedirectBlobStore) Exists(path string) (bool, error) {
	_, e := blobstore.s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if ae, isAwsErr := e.(awserr.Error); isAwsErr {
			ae.Code()
			if ae.Code() == "NoSuchBucket" || ae.Code() == "NoSuchKey" || ae.Code() == "NotFound" {
				return false, nil
			}
		}
		return false, errors.Wrapf(e, "Failed to check for %v/%v", blobstore.bucket, path)
	}
	return true, nil
}

func (blobstore *S3NoRedirectBlobStore) Delete(path string) error {
	_, e := blobstore.s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}
