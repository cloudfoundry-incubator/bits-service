package alibaba

import (
	"io"
	"strings"
	"time"

	"github.com/petergtz/bitsgo"
	"github.com/pkg/errors"

	oss "github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/petergtz/bitsgo/blobstores/validate"
	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/logger"
)

type Blobstore struct {
	BucketName string
	Client     *oss.Client
}

func NewBlobstore(config config.AlibabaBlobstoreConfig) *Blobstore {
	validate.NotEmpty(config.BucketName)
	validate.NotEmpty(config.ApiKey)
	validate.NotEmpty(config.ApiSecret)
	validate.NotEmpty(config.Endpoint)

	client, err := oss.New(config.Endpoint, config.ApiKey, config.ApiSecret)
	if err != nil {
		panic(err)
	}
	return &Blobstore{
		BucketName: config.BucketName,
		Client:     client,
	}
}

func (blobstore *Blobstore) Copy(src string, dest string) error {
	var copyErr error
	logger.Log.Debugw("Copy in Alibaba", "bucket", blobstore.BucketName, "src", src, "dest", dest)
	bucket := blobstore.getBucket()
	_, copyErr = bucket.CopyObjectFrom(blobstore.BucketName, src, dest)
	if copyErr != nil {
		return errors.Wrapf(copyErr, "Error while trying to copy src %v to dest %v in bucket %v", src, dest, blobstore.BucketName)
	}
	return copyErr
}

func (blobstore *Blobstore) Delete(resource string) error {
	bucket := blobstore.getBucket()
	return bucket.DeleteObject(resource)

}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	var deletionError error
	deletionErrs := []error{}
	bucket := blobstore.getBucket()
	prefixFilter := oss.Prefix(prefix)
	marker := oss.Marker("")
	portion := 20

	for {
		objList, err := bucket.ListObjects(oss.MaxKeys(portion), marker, prefixFilter)
		if err != nil {
			return errors.Wrapf(err, "Prefix %v", prefix)
		}
		deletionErrs = blobstore.deleteObjects(objList)
		marker = oss.Marker(objList.NextMarker)
		if !objList.IsTruncated {
			break
		}
	}

	if len(deletionErrs) < 0 {
		deletionError = errors.Errorf("Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	return deletionError
}

func (blobstore *Blobstore) deleteObjects(objListResult oss.ListObjectsResult) []error {
	bucket := blobstore.getBucket()
	deletionErrs := []error{}
	for _, obj := range objListResult.Objects {
		err := bucket.DeleteObject(obj.Key)
		if err != nil {
			deletionErrs = append(deletionErrs, err)
		}
	}
	return deletionErrs
}

func (blobstore *Blobstore) Exists(resource string) (bool, error) {
	bucket := blobstore.getBucket()
	return bucket.IsObjectExist(resource)
}

func (blobstore *Blobstore) Get(path string) (io.ReadCloser, error) {
	logger.Log.Debugw("GET", "bucket", blobstore.BucketName, "path", path)
	exists, _ := blobstore.Client.IsBucketExist(blobstore.BucketName)
	if !exists {
		return nil, errors.Errorf("Bucket not found: '%v'", blobstore.BucketName)
	}
	bucket := blobstore.getBucket()
	obj, err := bucket.GetObject(path)
	if err != nil {
		return nil, bitsgo.NewNotFoundErrorWithMessage("Could not find object: " + path)
	}
	return obj, nil
}

func (blobstore *Blobstore) GetOrRedirect(path string) (io.ReadCloser, string, error) {
	signedURL, err := blobstore.HeadOrRedirectAsGet(path)
	return nil, signedURL, err
}

func (blobstore *Blobstore) HeadOrRedirectAsGet(resource string) (string, error) {
	bucket := blobstore.getBucket()
	return bucket.SignURL(resource, oss.HTTPGet, getValidityPeriod(time.Now().Add(1*time.Hour)))
}

func (blobstore *Blobstore) Put(path string, rs io.ReadSeeker) error {
	logger.Log.Debugw("Put", "bucket", blobstore.BucketName, "path", path)
	exists, _ := blobstore.Client.IsBucketExist(blobstore.BucketName)
	if !exists {
		return errors.Errorf("Bucket not found: '%v'", blobstore.BucketName)
	}
	bucket := blobstore.getBucket()
	return bucket.PutObject(path, rs)
}

func (blobstore *Blobstore) Sign(resource string, method string, timestamp time.Time) string {
	var signedURL string
	bucket := blobstore.getBucket()
	switch strings.ToLower(method) {
	case "put":
		signedURL, _ = bucket.SignURL(resource, oss.HTTPPut, getValidityPeriod(timestamp))
	case "get":
		signedURL, _ = bucket.SignURL(resource, oss.HTTPGet, getValidityPeriod(timestamp))
	default:
		panic("Supported methods are 'put' and 'get'.Got'" + method + "'")
	}
	return signedURL
}

func getValidityPeriod(timestamp time.Time) int64 {
	duration := timestamp.Sub(time.Now()).Seconds()
	return int64(duration)
}

func (blobstore *Blobstore) getBucket() *oss.Bucket {
	bucket, err := blobstore.Client.Bucket(blobstore.BucketName)
	if err != nil {
		panic("Can't connect to bucket " + blobstore.BucketName)
	}
	return bucket
}
