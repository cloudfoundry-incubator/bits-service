package alibaba

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/bits-service"
	"github.com/pkg/errors"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/validate"
	"github.com/cloudfoundry-incubator/bits-service/config"
	"github.com/cloudfoundry-incubator/bits-service/logger"
)

type Blobstore struct {
	Client *oss.Client
	bucket *oss.Bucket
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
	bucket, e := client.Bucket(config.BucketName)
	if e != nil {
		panic(fmt.Errorf("could not get bucket"))
	}
	return &Blobstore{
		bucket: bucket,
		Client: client,
	}
}

func (blobstore *Blobstore) Copy(src string, dest string) error {
	logger.Log.Debugw("Copy in Alibaba", "bucket", blobstore.bucket.BucketName, "src", src, "dest", dest)
	_, e := blobstore.bucket.CopyObject(src, dest)
	if e != nil {
		return errors.Wrapf(e, "Error while trying to copy src %v to dest %v in bucket %v", src, dest, blobstore.bucket.BucketName)
	}
	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	e := blobstore.bucket.DeleteObject(path)
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	deletionErrs := []error{}
	marker := oss.Marker("")

	for {
		objList, e := blobstore.bucket.ListObjects(oss.MaxKeys(20), marker, oss.Prefix(prefix))
		if e != nil {
			return errors.Wrapf(e, "Prefix %v", prefix)
		}
		deletionErrs = append(deletionErrs, blobstore.deleteObjects(objList)...)
		marker = oss.Marker(objList.NextMarker)
		if !objList.IsTruncated {
			break
		}
	}

	if len(deletionErrs) > 0 {
		return errors.Errorf("Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	return nil
}

func (blobstore *Blobstore) deleteObjects(objListResult oss.ListObjectsResult) []error {
	deletionErrs := []error{}
	for _, obj := range objListResult.Objects {
		e := blobstore.bucket.DeleteObject(obj.Key)
		if e != nil {
			deletionErrs = append(deletionErrs, e)
		}
	}
	return deletionErrs
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	return blobstore.bucket.IsObjectExist(path)
}

func (blobstore *Blobstore) Get(path string) (io.ReadCloser, error) {
	logger.Log.Debugw("GET", "bucket", blobstore.bucket.BucketName, "path", path)
	exists, _ := blobstore.Client.IsBucketExist(blobstore.bucket.BucketName)
	if !exists {
		return nil, errors.Errorf("Bucket not found: '%v'", blobstore.bucket.BucketName)
	}
	obj, err := blobstore.bucket.GetObject(path)
	if err != nil {
		return nil, bitsgo.NewNotFoundErrorWithMessage("Could not find object: " + path)
	}
	return obj, nil
}

func (blobstore *Blobstore) GetOrRedirect(path string) (io.ReadCloser, string, error) {
	signedURL, err := blobstore.bucket.SignURL(path, oss.HTTPGet, getValidityPeriod(time.Now().Add(1*time.Hour)))
	return nil, signedURL, err
}

func (blobstore *Blobstore) Put(path string, rs io.ReadSeeker) error {
	logger.Log.Debugw("Put", "bucket", blobstore.bucket.BucketName, "path", path)
	exists, _ := blobstore.Client.IsBucketExist(blobstore.bucket.BucketName)
	if !exists {
		return errors.Errorf("Bucket not found: '%v'", blobstore.bucket.BucketName)
	}
	return blobstore.bucket.PutObject(path, rs)
}

func (blobstore *Blobstore) Sign(path string, method string, timestamp time.Time) string {
	var ossMethod oss.HTTPMethod
	switch strings.ToLower(method) {
	case "put":
		ossMethod = oss.HTTPPut
	case "get":
		ossMethod = oss.HTTPGet

	default:
		panic("Supported methods are 'put' and 'get'.Got'" + method + "'")
	}
	signedURL, e := blobstore.bucket.SignURL(path, ossMethod, getValidityPeriod(timestamp))
	if e != nil {
		panic(e)
	}
	return signedURL
}

func getValidityPeriod(timestamp time.Time) int64 {
	duration := timestamp.Sub(time.Now()).Seconds()
	return int64(duration)
}
