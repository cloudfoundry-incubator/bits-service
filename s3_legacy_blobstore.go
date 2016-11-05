package main

import (
	"io"
	"net/http"
	"net/url"
)

type S3LegacyBlobStore struct {
}

// func (blobstore *S3LegacyBlobStore) Exists(path string) bool {
// 	_, err := os.Stat(filepath.Join(blobstore.pathPrefix, path))
// 	return err == nil
// }
func (blobstore *S3LegacyBlobStore) Get(path string, responseWriter http.ResponseWriter) {
	http.Redirect(responseWriter, nil, blobstore.generatePreSignedS3Url(path), 302)
}

func (blobstore *S3LegacyBlobStore) Put(path string, src io.Reader, responseWriter http.ResponseWriter) {
	url, e := url.Parse(blobstore.s3PutRequest(path))
	if e != nil {
		responseWriter.WriteHeader(500)
		return
	}
	http.DefaultClient.Do(&http.Request{Method: http.MethodPut, URL: url})
}

// func (blobstore *S3LegacyBlobStore) Delete(path string) error {
// 	// TODO
// 	return nil
// }

func (blobstore *S3LegacyBlobStore) generatePreSignedS3Url(path string) string {
	return "http://pre-signed-s3-url"
}
func (blobstore *S3LegacyBlobStore) s3PutRequest(path string) string {
	return "http://pre-signed-s3-url"
}
