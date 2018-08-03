package azure

import (
	"encoding/base64"
	"io"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/Azure/go-autorest/autorest/azure"

	"net/http"

	"fmt"

	"github.com/cloudfoundry-incubator/bits-service"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/validate"
	"github.com/cloudfoundry-incubator/bits-service/config"
	"github.com/cloudfoundry-incubator/bits-service/logger"
	"github.com/pkg/errors"
)

type Blobstore struct {
	containerName  string
	client         storage.BlobStorageClient
	putBlockSize   int64
	maxListResults uint
}

func NewBlobstore(config config.AzureBlobstoreConfig) *Blobstore {
	return NewBlobstoreWithDetails(config, 50<<20, 5000)
}

func NewBlobstoreWithDetails(config config.AzureBlobstoreConfig, putBlockSize int64, maxListResults uint) *Blobstore {
	validate.NotEmpty(config.AccountKey)
	validate.NotEmpty(config.AccountName)
	validate.NotEmpty(config.ContainerName)
	validate.NotEmpty(config.EnvironmentName())

	environment, e := azure.EnvironmentFromName(config.EnvironmentName())
	if e != nil {
		logger.Log.Fatalw("Could not get Azure Environment from Name", "error", e, "environment", config.EnvironmentName())
	}
	client, e := storage.NewBasicClientOnSovereignCloud(config.AccountName, config.AccountKey, environment)
	if e != nil {
		logger.Log.Fatalw("Could not instantiate Azure Basic Client", "error", e)
	}
	return &Blobstore{
		client:         client.GetBlobService(),
		containerName:  config.ContainerName,
		putBlockSize:   putBlockSize,
		maxListResults: maxListResults,
	}
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	exists, e := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(path).Exists()
	if e != nil {
		return false, errors.Wrapf(e, "Failed to check for %v/%v", blobstore.containerName, path)
	}
	return exists, nil
}

func (blobstore *Blobstore) HeadOrRedirectAsGet(path string) (redirectLocation string, err error) {
	return blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(path).GetSASURI(time.Now().Add(time.Hour), "r")
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	logger.Log.Debugw("Get", "bucket", blobstore.containerName, "path", path)

	reader, e := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(path).Get(nil)
	if e != nil {
		return nil, blobstore.handleError(e, "Path %v", path)
	}
	return reader, nil
}

func (blobstore *Blobstore) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	signedUrl, e := blobstore.HeadOrRedirectAsGet(path)
	return nil, signedUrl, e
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	logger.Log.Debugw("Put", "bucket", blobstore.containerName, "path", path)
	blob := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(path)

	e := blob.CreateBlockBlob(nil)
	if e != nil {
		return errors.Wrapf(e, "create block blob failed. container: %v, path: %v", blobstore.containerName, path)
	}

	uncommittedBlocksList := make([]storage.Block, 0)
	eof := false
	for i := 0; !eof; i++ {
		// using information from https://docs.microsoft.com/en-us/rest/api/storageservices/understanding-block-blobs--append-blobs--and-page-blobs
		block := storage.Block{
			ID:     base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%05d", i))),
			Status: storage.BlockStatusUncommitted,
		}
		data := make([]byte, blobstore.putBlockSize)
		numBytesRead, e := src.Read(data)
		if e != nil {
			if e.Error() == "EOF" {
				eof = true
			} else {
				return errors.Wrapf(e, "put block failed: %v", path)
			}
		}
		if numBytesRead == 0 {
			continue
		}
		e = blob.PutBlock(block.ID, data[:numBytesRead], nil)
		if e != nil {
			return errors.Wrapf(e, "put block failed: %v", path)
		}
		uncommittedBlocksList = append(uncommittedBlocksList, block)
	}
	e = blob.PutBlockList(uncommittedBlocksList, nil)
	if e != nil {
		return errors.Wrapf(e, "put block list failed: %v", path)
	}

	return nil
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	logger.Log.Debugw("Copy in Azure", "container", blobstore.containerName, "src", src, "dest", dest)
	e := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(dest).Copy(src, nil)

	if e != nil {
		blobstore.handleError(e, "Error while trying to copy src %v to dest %v in bucket %v", src, dest, blobstore.containerName)
	}
	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	deleted, e := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(path).DeleteIfExists(nil)
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}
	if !deleted {
		return bitsgo.NewNotFoundError()
	}
	return nil
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	deletionErrs := []error{}
	marker := ""
	for {
		response, e := blobstore.client.GetContainerReference(blobstore.containerName).ListBlobs(storage.ListBlobsParameters{
			Prefix:     prefix,
			MaxResults: blobstore.maxListResults,
			Marker:     marker,
		})
		if e != nil {
			return errors.Wrapf(e, "Prefix %v", prefix)
		}
		for _, blob := range response.Blobs {
			e = blobstore.Delete(blob.Name)
			if e != nil {
				if _, isNotFoundError := e.(*bitsgo.NotFoundError); !isNotFoundError {
					deletionErrs = append(deletionErrs, e)
				}
			}
		}
		if response.NextMarker == "" {
			break
		}
		marker = response.NextMarker
	}

	if len(deletionErrs) != 0 {
		return errors.Errorf("Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	return nil
}

func (blobstore *Blobstore) Sign(resource string, method string, expirationTime time.Time) (signedURL string) {
	var e error
	switch strings.ToLower(method) {
	case "put":
		signedURL, e = blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(resource).GetSASURI(expirationTime, "wc")
	case "get":
		signedURL, e = blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(resource).GetSASURI(expirationTime, "r")
	default:
		panic("The only supported methods are 'put' and 'get'")
	}
	if e != nil {
		panic(e)
	}
	return
}

func (blobstore *Blobstore) handleError(e error, context string, args ...interface{}) error {
	if azse, ok := e.(storage.AzureStorageServiceError); ok && azse.StatusCode == http.StatusNotFound {
		exists, e := blobstore.client.GetContainerReference(blobstore.containerName).Exists()
		if e != nil {
			return errors.Wrapf(e, context, args...)
		}
		if !exists {
			return errors.Errorf("Container does not exist '%v", blobstore.containerName)
		}
		return bitsgo.NewNotFoundError()
	}
	return errors.Wrapf(e, context, args...)
}
