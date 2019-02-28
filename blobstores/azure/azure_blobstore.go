package azure

import (
	"encoding/base64"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"github.com/cenkalti/backoff"

	"github.com/Azure/go-autorest/autorest"

	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/Azure/go-autorest/autorest/azure"

	"net/http"

	"fmt"

	bitsgo "github.com/cloudfoundry-incubator/bits-service"
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
	metricsService bitsgo.MetricsService
}

func NewBlobstore(config config.AzureBlobstoreConfig, metricsService bitsgo.MetricsService) *Blobstore {
	return NewBlobstoreWithDetails(config, 50<<20, 5000, metricsService)
}

// NetworkErrorRetryingSender is a replacement for the storage.DefaultSender.
// storage.DefaultSender does not retry on network errors which is rarely what we want in
// a production system.
type NetworkErrorRetryingSender struct{}

var retryableStatusCodes = []int{
	http.StatusRequestTimeout,      // 408
	http.StatusInternalServerError, // 500
	http.StatusBadGateway,          // 502
	http.StatusServiceUnavailable,  // 503
	http.StatusGatewayTimeout,      // 504
}

// Send is mostly emulating storage.DefaultSender.Send, so most code was copied.
// But we use the backoff library for convenience and retry on errors returned from
// HTTPClient.Do.
func (sender *NetworkErrorRetryingSender) Send(c *storage.Client, req *http.Request) (*http.Response, error) {
	rr := autorest.NewRetriableRequest(req)
	var resp *http.Response

	err := backoff.Retry(func() error {
		err := rr.Prepare()
		if err != nil {
			return backoff.Permanent(err)
		}
		resp, err = c.HTTPClient.Do(rr.Request())
		// We deliberately mark errors *not* as permanent, because an error
		// here means network connectivity or similar. This is different to the
		// storage.DefaultSender which stops on any error.
		if err != nil {
			return err
		}
		if !autorest.ResponseHasStatusCode(resp, retryableStatusCodes...) {
			return backoff.Permanent(err)
		}
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5))

	return resp, err
}

func NewBlobstoreWithDetails(config config.AzureBlobstoreConfig, putBlockSize int64, maxListResults uint, metricsService bitsgo.MetricsService) *Blobstore {
	validate.NotEmpty(config.AccountKey)
	validate.NotEmpty(config.AccountName)
	validate.NotEmpty(config.ContainerName)
	validate.NotEmpty(config.EnvironmentName())
	if metricsService == nil {
		panic("metricsService must not be nil")
	}

	environment, e := azure.EnvironmentFromName(config.EnvironmentName())
	if e != nil {
		logger.Log.Fatalw("Could not get Azure Environment from Name", "error", e, "environment", config.EnvironmentName())
	}
	client, e := storage.NewBasicClientOnSovereignCloud(config.AccountName, config.AccountKey, environment)
	if e != nil {
		logger.Log.Fatalw("Could not instantiate Azure Basic Client", "error", e)
	}
	client.Sender = &NetworkErrorRetryingSender{}
	return &Blobstore{
		client:         client.GetBlobService(),
		containerName:  config.ContainerName,
		putBlockSize:   putBlockSize,
		maxListResults: maxListResults,
		metricsService: metricsService,
	}
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	exists, e := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(path).Exists()
	if e != nil {
		return false, errors.Wrapf(e, "Failed to check for %v/%v", blobstore.containerName, path)
	}
	return exists, nil
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
	signedUrl, e := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(path).GetSASURI(storage.BlobSASOptions{
		BlobServiceSASPermissions: storage.BlobServiceSASPermissions{Read: true},
		SASOptions:                storage.SASOptions{Expiry: time.Now().Add(time.Hour)},
	})
	return nil, signedUrl, e
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	putRequestID := rand.Int63()
	l := logger.Log.With("put-request-id", putRequestID)
	l.Debugw("Put", "bucket", blobstore.containerName, "path", path)

	blob := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(path)

	e := blob.CreateBlockBlob(nil)
	if e != nil {
		return errors.Wrapf(e, "create block blob failed. container: %v, path: %v, put-request-id: %v", blobstore.containerName, path, putRequestID)
	}

	uncommittedBlocksList := make([]storage.Block, 0)
	eof := false
	for i := 0; !eof; i++ {
		// using information from https://docs.microsoft.com/en-us/rest/api/storageservices/understanding-block-blobs--append-blobs--and-page-blobs
		if i >= 50000 {
			return errors.Errorf("block blob cannot have more than 50,000 blocks. path: %v, put-request-id: %v", path, putRequestID)
		}
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
				return errors.Wrapf(e, "put block failed. path: %v, put-request-id: %v", path, putRequestID)
			}
		}
		if numBytesRead == 0 {
			l.Debugw("Empty read", "block-index", i, "block-id", block.ID, "is-eof", eof)
			continue
		}
		l.Debugw("PutBlock", "block-index", i, "block-id", block.ID, "block-size", numBytesRead, "is-eof", eof)
		e = backoff.RetryNotify(func() error {
			return blob.PutBlock(block.ID, data[:numBytesRead], nil)
		}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 2), func(error, time.Duration) {
			l.Infow("Retry PutBlock", "block-index", i, "block-id", block.ID, "block-size", numBytesRead, "is-eof", eof)
			blobstore.metricsService.SendCounterMetric("put-block-retry", 1)
		})
		if e != nil {
			return errors.Wrapf(e, "put block failed. path: %v, put-request-id: %v", path, putRequestID)
		}
		uncommittedBlocksList = append(uncommittedBlocksList, block)
	}
	l.Debugw("PutBlockList", "uncommitted-block-list", uncommittedBlocksList)
	e = backoff.RetryNotify(func() error {
		return blob.PutBlockList(uncommittedBlocksList, nil)
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 2), func(error, time.Duration) {
		l.Infow("Retry PutBlockList", "uncommitted-block-list", uncommittedBlocksList)
		blobstore.metricsService.SendCounterMetric("put-block-list-retry", 1)
	})
	if e != nil {
		return errors.Wrapf(e, "put block list failed. path: %v, put-request-id: %v", path, putRequestID)
	}

	return nil
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	logger.Log.Debugw("Copy in Azure", "container", blobstore.containerName, "src", src, "dest", dest)
	e := blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(dest).Copy(
		blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(src).GetURL(), nil)

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
		signedURL, e = blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(resource).GetSASURI(storage.BlobSASOptions{
			BlobServiceSASPermissions: storage.BlobServiceSASPermissions{Write: true, Create: true},
			SASOptions:                storage.SASOptions{Expiry: expirationTime},
		})
	case "get":
		signedURL, e = blobstore.client.GetContainerReference(blobstore.containerName).GetBlobReference(resource).GetSASURI(storage.BlobSASOptions{
			BlobServiceSASPermissions: storage.BlobServiceSASPermissions{Read: true},
			SASOptions:                storage.SASOptions{Expiry: expirationTime},
		})
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
