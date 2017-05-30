package azure

import (
	"encoding/base64"
	"io"
	"time"

	"github.com/Azure/azure-sdk-for-go/storage"

	"net/http"

	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/logger"
	"github.com/pkg/errors"
)

type Blobstore struct {
	containerName string
	client        storage.BlobStorageClient
}

func NewBlobstore(config config.AzureBlobstoreConfig) *Blobstore {
	client, e := storage.NewBasicClient(config.AccountName, config.AccountKey)
	if e != nil {
		panic(e)
	}
	return &Blobstore{
		client:        client.GetBlobService(),
		containerName: config.ContainerName,
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
	logger.Log.Debugw("Get from azure", "bucket", blobstore.containerName, "path", path)

	reader, e := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(path).Get(nil)
	if e != nil {
		if azse, ok := e.(storage.AzureStorageServiceError); ok && azse.StatusCode == http.StatusNotFound {
			return nil, bitsgo.NewNotFoundError()
		}
		return nil, errors.Wrapf(e, "Path %v", path)
	}
	return reader, nil
}

func (blobstore *Blobstore) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	signedUrl, e := blobstore.HeadOrRedirectAsGet(path)
	return nil, signedUrl, e
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	logger.Log.Debugw("Put to GCP", "bucket", blobstore.containerName, "path", path)
	blob := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(path)

	e := blob.CreateBlockBlob(nil)
	if e != nil {
		return errors.Wrapf(e, "create block blob failed. container: %v, path: %v", blobstore.containerName, path)
	}

	uncommittedBlocksList := make([]storage.Block, 0)
	eof := false
	for i := 0; !eof; i++ {
		logger.Log.Debugw("Put a block...")
		blockID := base64.StdEncoding.EncodeToString([]byte("00000"))
		data := make([]byte, 50*1024*1024)
		dataSize, e := src.Read(data)
		if e != nil {
			if e.Error() == "EOF" {
				eof = true
			} else {
				panic(e)
			}
		}
		if dataSize == 0 {
			continue
		}
		e = blob.PutBlock(blockID, data[:dataSize], nil)
		if e != nil {
			return errors.Wrapf(e, "put block failed: %v", path)
		}
		uncommittedBlocksList = append(uncommittedBlocksList, storage.Block{ID: blockID, Status: storage.BlockStatusUncommitted})
	}
	logger.Log.Debugw("Commit blocks...")
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
		if azse, ok := e.(storage.AzureStorageServiceError); ok && azse.StatusCode == http.StatusNotFound {
			return bitsgo.NewNotFoundError()
		}
		return errors.Wrapf(e, "Error while trying to copy src %v to dest %v in bucket %v", src, dest, blobstore.containerName)
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
	response, e := blobstore.client.GetContainerReference(blobstore.containerName).ListBlobs(storage.ListBlobsParameters{
		Prefix: prefix,
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
	if len(deletionErrs) != 0 {
		return errors.Errorf("Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	return nil
}
