package s3_blobstore

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
)

type S3LegacyBlobStore struct {
	s3Client *s3.S3
	bucket   string
}

func NewS3LegacyBlobstore(bucket string, accessKeyID, secretAccessKey string) *S3LegacyBlobStore {
	return &S3LegacyBlobStore{
		s3Client: newS3Client(DefaultS3Region, accessKeyID, secretAccessKey),
		bucket:   bucket,
	}
}

func (blobstore *S3LegacyBlobStore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(5 * time.Second)
	if e != nil {
		panic(e)
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
		return "", fmt.Errorf("TODO %v", e)
	}
	return "", nil
}

func (blobstore *S3LegacyBlobStore) Exists(path string) (bool, error) {
	panic("TODO")
}

type S3PureRedirectBlobStore struct {
	s3Client *s3.S3
	bucket   string
}

func NewS3PureRedirectBlobstore(bucket string, accessKeyID, secretAccessKey string) *S3PureRedirectBlobStore {
	return &S3PureRedirectBlobStore{
		s3Client: newS3Client(DefaultS3Region, accessKeyID, secretAccessKey),
		bucket:   bucket,
	}
}

func (blobstore *S3PureRedirectBlobStore) Get(path string) (statusCode int, body io.ReadCloser, header map[string][]string) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(5 * time.Second)
	if e != nil {
		panic(e)
	}
	return http.StatusFound, nil, map[string][]string{"Location": []string{signedURL}}
}

func (blobstore *S3PureRedirectBlobStore) Put(path string, src io.Reader) (statusCode int, header map[string][]string) {
	request, _ := blobstore.s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(5 * time.Second)
	if e != nil {
		panic(e)
	}
	return http.StatusFound, map[string][]string{"Location": []string{signedURL}}
}
