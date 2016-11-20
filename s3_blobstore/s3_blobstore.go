package s3_blobstore

import (
	"io"
	"log"
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

func (blobstore *S3LegacyBlobStore) Get(path string, responseWriter http.ResponseWriter) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(5 * time.Second)
	if e != nil {
		panic(e)
	}
	r, e := http.NewRequest("GET", path, nil)
	if e != nil {
		panic(e)
	}
	http.Redirect(responseWriter, r, signedURL, 302)
}

func (blobstore *S3LegacyBlobStore) Put(path string, src io.ReadSeeker, responseWriter http.ResponseWriter) {
	_, e := blobstore.s3Client.PutObject(&s3.PutObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
		Body:   src,
	})
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	responseWriter.WriteHeader(201)
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

func (blobstore *S3PureRedirectBlobStore) Get(path string, responseWriter http.ResponseWriter) {
	request, _ := blobstore.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(5 * time.Second)
	if e != nil {
		panic(e)
	}
	r, e := http.NewRequest("GET", path, nil)
	if e != nil {
		panic(e)
	}
	http.Redirect(responseWriter, r, signedURL, 302)
}

func (blobstore *S3PureRedirectBlobStore) Put(path string, src io.Reader, responseWriter http.ResponseWriter) {
	request, _ := blobstore.s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: &blobstore.bucket,
		Key:    &path,
	})
	signedURL, e := request.Presign(5 * time.Second)
	if e != nil {
		panic(e)
	}
	r, e := http.NewRequest("GET", path, nil)
	if e != nil {
		panic(e)
	}
	http.Redirect(responseWriter, r, signedURL, 302)
}
