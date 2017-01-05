package s3_blobstore

import (
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/petergtz/bitsgo/routes"
	"github.com/pkg/errors"
)

type S3LegacyBlobStore struct {
	s3Client *s3.S3
	bucket   string
}

func NewS3LegacyBlobstore(bucket, accessKeyID, secretAccessKey, region string) *S3LegacyBlobStore {
	return &S3LegacyBlobStore{
		s3Client: newS3Client(region, accessKeyID, secretAccessKey),
		bucket:   bucket,
	}
}

func (blobstore *S3LegacyBlobStore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(time.Hour)
	if e != nil {
		return nil, "", errors.Wrapf(e, "Path %v", path)
	}
	return nil, signedURL, nil
}

func (blobstore *S3LegacyBlobStore) Put(path string, src io.ReadSeeker) (redirectLocation string, err error) {
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

func (blobstore *S3LegacyBlobStore) Exists(path string) (bool, error) {
	_, e := blobstore.s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		if ae, isAwsErr := e.(awserr.Error); isAwsErr {
			if ae.Code() == "NoSuchBucket" || ae.Code() == "NoSuchKey" || ae.Code() == "NotFound" {
				return false, nil
			}
		}
		return false, errors.Wrapf(e, "Failed to check for %v/%v", blobstore.bucket, path)
	}
	return true, nil
}

func (blobstore *S3LegacyBlobStore) Delete(path string) error {
	_, e := blobstore.s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	if e != nil {
		return errors.Wrapf(e, "Failed to delete %v/%v", blobstore.bucket, path)
	}
	return nil
}

type S3PureRedirectBlobStore struct {
	s3Client *s3.S3
	bucket   string
}

func NewS3PureRedirectBlobstore(bucket, accessKeyID, secretAccessKey, region string) *S3PureRedirectBlobStore {
	return &S3PureRedirectBlobStore{
		s3Client: newS3Client(region, accessKeyID, secretAccessKey),
		bucket:   bucket,
	}
}

func (blobstore *S3PureRedirectBlobStore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(time.Hour)
	if e != nil {
		return nil, "", errors.Wrapf(e, "Path %v", path)
	}
	return nil, signedURL, nil
}

func (blobstore *S3PureRedirectBlobStore) Put(path string, src io.Reader) (redirectLocation string, err error) {
	request, _ := blobstore.s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(time.Hour)
	if e != nil {
		return "", errors.Wrapf(e, "Path %v", path)
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

func NewS3NoRedirectBlobStore(bucket, accessKeyID, secretAccessKey, region string) *S3NoRedirectBlobStore {
	return &S3NoRedirectBlobStore{
		s3Client: newS3Client(region, accessKeyID, secretAccessKey),
		bucket:   bucket,
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
