package azure

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	a "github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/mattetti/filebuffer"

	bitsgo "github.com/cloudfoundry-incubator/bits-service"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/validate"
	"github.com/cloudfoundry-incubator/bits-service/config"
	"github.com/cloudfoundry-incubator/bits-service/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	megaByte         = 1 << 20
	defaultBlockSize = 50 * megaByte
	maxRetries       = 5
	retryDelay       = 1 * time.Second
)

type Blobstore struct {
	metricsService bitsgo.MetricsService
	credential     *a.SharedKeyCredential
	serviceURL     a.ServiceURL
	containerURL   a.ContainerURL
	pipeline       pipeline.Pipeline
	containerName  string
	putBlockSize   int
	maxListResults uint
	ctx            context.Context
}

func NewBlobstore(config config.AzureBlobstoreConfig, metricsService bitsgo.MetricsService) *Blobstore {
	return NewBlobstoreWithDetails(config, defaultBlockSize, 5000, metricsService)
}

func NewBlobstoreWithDetails(config config.AzureBlobstoreConfig, putBlockSize int64, maxListResults uint, metricsService bitsgo.MetricsService) *Blobstore {
	validate.NotEmpty(config.AccountKey)
	validate.NotEmpty(config.AccountName)
	validate.NotEmpty(config.ContainerName)
	validate.NotEmpty(config.EnvironmentName())
	if metricsService == nil {
		panic("metricsService must not be nil")
	}

	if putBlockSize > math.MaxInt32 {
		logger.Log.Fatalw("putBlockSize must fit into an int32", "putBlockSize", putBlockSize)
		return nil
	}
	if putBlockSize > a.BlockBlobMaxStageBlockBytes {
		logger.Log.Fatalw("putBlockSize must be equal or less than 100 MB", "putBlockSize", putBlockSize)
		return nil
	}
	sharedKeyCredential, e := a.NewSharedKeyCredential(config.AccountName, config.AccountKey)
	if e != nil {
		logger.Log.Fatalw("Could not instantiate SharedKeyCredential", "error", e)
	}
	pipeline := a.NewPipeline(sharedKeyCredential, a.PipelineOptions{
		Retry: a.RetryOptions{
			Policy:     a.RetryPolicyExponential,
			MaxTries:   maxRetries,
			RetryDelay: retryDelay,
		}})

	url, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", config.AccountName))
	serviceURL := a.NewServiceURL(*url, pipeline)
	containerURL := serviceURL.NewContainerURL(config.ContainerName)

	return &Blobstore{
		metricsService: metricsService,
		credential:     sharedKeyCredential,
		pipeline:       pipeline,
		serviceURL:     serviceURL,
		containerURL:   containerURL,
		containerName:  config.ContainerName,
		putBlockSize:   int(putBlockSize),
		maxListResults: maxListResults,
		ctx:            context.Background(),
	}
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	blockBlobURL := blobstore.containerURL.NewBlockBlobURL(path)
	if _, e := blockBlobURL.GetProperties(blobstore.ctx, a.BlobAccessConditions{}); e != nil {
		if se, ok := e.(a.StorageError); ok && se.ServiceCode() == a.ServiceCodeBlobNotFound {
			return false, nil
		}
		return false, errors.Wrapf(e, "Failed to check for %s", blockBlobURL)
	}
	return true, nil
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	blockBlobURL := blobstore.containerURL.NewBlockBlobURL(path)
	logger.Log.Debugw("Get", "blob", blockBlobURL)

	if response, e := blockBlobURL.Download(blobstore.ctx, 0, a.CountToEnd, a.BlobAccessConditions{}, false); e != nil {
		return nil, errors.Wrapf(e, "Failed to download for %s", blockBlobURL)
	} else {
		return response.Body(a.RetryReaderOptions{}), nil
	}
}

func (blobstore *Blobstore) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	defer func() {
		if e := recover(); e != nil {
			body = nil
			redirectLocation = ""
			err = fmt.Errorf(e.(string))
		}
	}()

	expiry := time.Now().UTC().Add(1 * time.Hour)
	return nil, blobstore.Sign(path, "GET", expiry), nil
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	//
	// Upload strategy:
	// Determine, whether the upload is needed. It's needed if the file is missing, or the size differs, or if the contents differ or we don't know whether contents differ.
	// If upload is needed, do the following:
	// It can happen that multiple clients request uploads into the same blob.
	// To prevent collisions, each client uploads to a unique file first, then copies to the final blob, and deletes the original upload.
	//
	var (
		putRequestID = rand.Int63()
		l            = logger.Log.With("put-request-id", putRequestID)
		ctx          = blobstore.ctx
	)

	exists, blobContentLength, blobContentMD5, e := blobstore.getBlobProps(path)
	if e != nil {
		return errors.Wrapf(e, "Could not reach Azure blob storage. path: %v, put-request-id: %v", path, putRequestID)
	}
	l.Debugw("Put", "bucket", blobstore.containerName, "path", path, "exists", exists, "blobContentLength", blobContentLength, "blobContentMD5", blobContentMD5)

	uploadRequired, sourceContentLength := uploadIsRequired(src, blobContentLength, blobContentMD5)
	l.Infow("Put", "bucket", blobstore.containerName, "path", path, "uploadRequired", uploadRequired)
	if !uploadRequired {
		return nil
	}

	pathWithRequestIDSuffix := fmt.Sprintf("%s_%d", path, putRequestID)
	l.Debugw("Put", "bucket", blobstore.containerName, "path", path, "pathWithRequestIDSuffix ", pathWithRequestIDSuffix)

	temporaryBlob := blobstore.containerURL.NewBlockBlobURL(pathWithRequestIDSuffix)
	overallMD5 := md5.New()
	numberOfBlocks, err := getNumberOfBlocks(sourceContentLength, blobstore.putBlockSize)
	if err != nil {
		return errors.Wrapf(err, "PutBlock() failed. path: %v, pathWithRequestIDSuffix : %v, put-request-id: %v", path, pathWithRequestIDSuffix, putRequestID)
	}

	l.Debugw("Put", "bucket", blobstore.containerName, "path", path, "sourceContentLength", sourceContentLength, "numberOfBlocks", numberOfBlocks, "blobstore.putBlockSize", blobstore.putBlockSize)

	uncommittedBlocksList := make([]string, numberOfBlocks)
	for i := 0; i < numberOfBlocks; i++ {
		uncommittedBlocksList[i] = blockIDfromIndex(i)
	}

	data := make([]byte, blobstore.putBlockSize)
	for i := 0; i <= numberOfBlocks; i++ {
		expectedByteCount := func() int {
			if i < numberOfBlocks-1 {
				return blobstore.putBlockSize
			}
			return int(sourceContentLength % int64(blobstore.putBlockSize))
		}()

		t1 := time.Now()
		// make sure we upload full blocks to Azure storage
		numBytesRead, e := io.ReadAtLeast(src, data, expectedByteCount)
		duration := time.Since(t1).Seconds()
		l.Debugw("Put", "i", i, "disk-read-bytes", numBytesRead, "disk-read-duration", duration, "disk-read-throughput", fmt.Sprintf("%.2f MB/s", float64(numBytesRead)/(duration*megaByte)))

		if i == numberOfBlocks && numBytesRead == 0 && e == io.EOF {
			// now we have reached EOF
			break
		} else if e != nil {
			return errors.Wrapf(e, "put block failed. path: %v, put-request-id: %v", path, putRequestID)
		}

		bytesToUpload := data[:numBytesRead]
		cloned := append(bytesToUpload[:0:0], bytesToUpload...)
		overallMD5.Write(cloned)
		if putBlockErr := uploadSingleBlock(ctx, blobstore, temporaryBlob, l, cloned, i); putBlockErr != nil {
			return wrapWithAzureDetails(putBlockErr, fmt.Sprintf("PutBlock() failed. path: %v, pathWithRequestIDSuffix : %v, put-request-id: %v", path, pathWithRequestIDSuffix, putRequestID))
		}
	}

	l.Debugw("PutBlockList", "uncommitted-block-list", debugBlockIDAsString(uncommittedBlocksList))
	if _, putBlockListErr := temporaryBlob.CommitBlockList(blobstore.ctx, uncommittedBlocksList, a.BlobHTTPHeaders{ContentMD5: overallMD5.Sum(nil)}, a.Metadata{}, a.BlobAccessConditions{}); putBlockListErr != nil {
		return wrapWithAzureDetails(putBlockListErr, fmt.Sprintf("PutBlockList() failed. path: %v, pathWithRequestIDSuffix : %v, put-request-id: %v", path, pathWithRequestIDSuffix, putRequestID))
	}
	defer func() {
		if _, err := temporaryBlob.Delete(ctx, a.DeleteSnapshotsOptionNone, a.BlobAccessConditions{}); err != nil {
			l.Errorf("Error deleting temporary blob: %s", err)
			return
		}
	}()

	blob := blobstore.containerURL.NewBlockBlobURL(path)
	if _, copyErr := blob.StartCopyFromURL(
		ctx, temporaryBlob.URL(), a.Metadata{}, a.ModifiedAccessConditions{}, a.BlobAccessConditions{},
	); copyErr != nil {
		return wrapWithAzureDetails(copyErr, fmt.Sprintf("Copy() failed. path: %v, pathWithRequestIDSuffix : %v", path, pathWithRequestIDSuffix))
	}
	l.Debugw("Copy", "source-blob", temporaryBlob, "destination-blob", blob)

	return nil
}

func uploadSingleBlock(ctx context.Context, blobstore *Blobstore, temporaryBlob a.BlockBlobURL, l *zap.SugaredLogger, bytesToUpload []byte, i int) error {
	blockID := blockIDfromIndex(i)

	checkSumMD5 := md5.New()
	checkSumMD5.Write(bytesToUpload)
	blockMD5bytes := checkSumMD5.Sum(nil)
	length := len(bytesToUpload)

	t1 := time.Now()
	body := filebuffer.New(bytesToUpload)
	response, err := temporaryBlob.StageBlock(ctx, blockID, body, a.LeaseAccessConditions{}, blockMD5bytes)
	duration := time.Since(t1).Seconds()
	l.Debugw("Put", "i", i, "azure-upload-bytes", length, "azure-upload-duration", duration, "azure-upload-throughput", fmt.Sprintf("%.2f MB/sec", float64(length)/(duration*megaByte)), "status", response.Status())
	return err
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	srcURL := blobstore.containerURL.NewBlockBlobURL(src)
	destURL := blobstore.containerURL.NewBlockBlobURL(dest)

	t1 := time.Now()
	response, e := destURL.StartCopyFromURL(blobstore.ctx, srcURL.URL(), a.Metadata{}, a.ModifiedAccessConditions{}, a.BlobAccessConditions{})
	logger.Log.Debugw("Copy", "src", src, "dest", dest, "duration-in-seconds", time.Since(t1).Seconds())

	if e != nil {
		blobstore.handleError(e, "Error while trying to copy src %v to dest %v in bucket %v", src, dest, blobstore.containerName)
	}

	logger.Log.Debugw("Copy", "container", blobstore.containerName, "src", src, "dest", dest, "copy-id", response.CopyID(), "response-status", response.Status())
	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	blockBlobURL := blobstore.containerURL.NewBlockBlobURL(path)

	t1 := time.Now()
	response, e := blockBlobURL.Delete(blobstore.ctx, a.DeleteSnapshotsOptionInclude, a.BlobAccessConditions{})
	logger.Log.Debugw("Delete", "path", path, "duration-in-seconds", time.Since(t1).Seconds())

	if e != nil {
		if se, ok := e.(a.StorageError); ok && se.ServiceCode() == a.ServiceCodeBlobNotFound {
			// Blob was already deleted
			return nil
		}
		logger.Log.Debugw("Delete", "container", blobstore.containerName, "path", path, "error-code", response.ErrorCode())
		return errors.Wrapf(e, "Failed to delete %s", blockBlobURL)
	} else {
		return nil
	}
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	deletionErrs := make([]error, 0)
	const delimiter = "/"
	marker := a.Marker{}

	for {
		response, e := blobstore.containerURL.ListBlobsHierarchySegment(blobstore.ctx, marker, delimiter, a.ListBlobsSegmentOptions{Prefix: prefix})
		if e != nil {
			return errors.Wrapf(e, "Prefix %v", prefix)
		}

		for _, blob := range response.Segment.BlobItems {
			t1 := time.Now()
			e = blobstore.Delete(blob.Name)
			logger.Log.Debugw("DeleteDir", "prefix", prefix, "blob-name", blob.Name, "duration-in-seconds", time.Since(t1).Seconds())

			if e != nil {
				if _, ok := e.(a.StorageError); !ok {
					deletionErrs = append(deletionErrs, e)
				}
			}
		}

		marker = response.NextMarker
		if marker.NotDone() {
			continue
		} else {
			break
		}
	}

	if len(deletionErrs) != 0 {
		return errors.Errorf("Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	return nil
}

func (blobstore *Blobstore) Sign(path string, method string, expirationTime time.Time) (signedURL string) {
	blockBlobURL := blobstore.containerURL.NewBlockBlobURL(path)

	permsFromMethod := func(m string) a.BlobSASPermissions {
		switch strings.ToLower(m) {
		case "put":
			return a.BlobSASPermissions{Write: true, Create: true}
		case "get":
			return a.BlobSASPermissions{Read: true}
		default:
			panic("The only supported methods are 'put' and 'get'")
		}
	}

	qp, e := a.BlobSASSignatureValues{
		Protocol:      a.SASProtocolHTTPS,
		ExpiryTime:    expirationTime,
		Permissions:   permsFromMethod(method).String(),
		ContainerName: blobstore.containerName,
		BlobName:      path,
	}.NewSASQueryParameters(blobstore.credential)

	if e != nil {
		panic(e)
	}

	return fmt.Sprintf("%s?%s", blockBlobURL, qp.Encode())
}

func (blobstore *Blobstore) handleError(e error, context string, args ...interface{}) error {
	if azse, ok := e.(a.StorageError); ok {
		if azse.ServiceCode() == a.ServiceCodeContainerNotFound {
			return errors.Errorf("Container does not exist '%v", blobstore.containerName)
		}
		return bitsgo.NewNotFoundError()
	}
	return errors.Wrapf(e, context, args...)
}

func (blobstore *Blobstore) getBlobProps(path string) (exists bool, blobContentLength int64, blobContentMD5 string, err error) {
	blockBlobURL := blobstore.containerURL.NewBlockBlobURL(path)
	props, err := blockBlobURL.GetProperties(blobstore.ctx, a.BlobAccessConditions{})
	if err != nil {
		exists = false
		blobContentLength = -1
		blobContentMD5 = ""

		if azse, ok := err.(a.StorageError); ok && azse.ServiceCode() == a.ServiceCodeBlobNotFound {
			err = nil
		}
		return
	}
	exists = true
	blobContentLength = props.ContentLength()
	blobContentMD5 = base64.StdEncoding.EncodeToString(props.ContentMD5())
	return
}

func uploadIsRequired(src io.ReadSeeker, blobContentLength int64, blobContentMD5 string) (uploadRequired bool, sourceContentLength int64) {
	// Check if upload is actually required
	sourceContentLength, err := src.Seek(0, io.SeekEnd)
	defer src.Seek(0, io.SeekStart)
	if err != nil {
		// if the blob can't be found -> upload
		uploadRequired = true
		return
	}
	if sourceContentLength != blobContentLength {
		// if the length differs -> upload
		uploadRequired = true
		return
	}
	if blobContentMD5 == "" {
		// if we cannot check the MD5 -> upload
		uploadRequired = true
		return
	}
	src.Seek(0, io.SeekStart)
	hash := md5.New()
	io.Copy(hash, src)
	localResourceMD5 := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	// if the MD5 differs -> upload
	uploadRequired = blobContentMD5 != localResourceMD5
	return
}

func getNumberOfBlocks(inputLength int64, blockSize int) (int, error) {
	fullyFilledBlocks := int(inputLength / int64(blockSize))
	hasPartiallyFilledBlock := (inputLength % int64(blockSize)) > 0
	var numberOfBlocks int
	if hasPartiallyFilledBlock {
		numberOfBlocks = fullyFilledBlocks + 1
	} else {
		numberOfBlocks = fullyFilledBlocks
	}
	if numberOfBlocks > a.BlockBlobMaxBlocks {
		return -1, errors.Errorf("BlockBlob cannot have more than %d blocks. File size %v bytes, block size %d", a.BlockBlobMaxBlocks, inputLength, blockSize)
	}
	return numberOfBlocks, nil
}

func blockIDfromIndex(i int) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%05d", i)))
}

func wrapWithAzureDetails(e error, message string) error {
	if e == nil {
		return nil
	}
	// if azse, ok := e.(a.StorageError); ok {
	// 	return errors.Wrapf(e, "%s x-ms-request-id: %s", message, azse.RequestID)
	// }
	return errors.Wrapf(e, "%s", message)
}

func debugBlockIDAsString(l []string) string {
	length := len(l)
	if length == 0 {
		return fmt.Sprintf("[] (%d blocks overall)", length)
	} else if length == 1 {
		return fmt.Sprintf("[%v] (%d blocks overall)", l[0], length)
	} else if length == 2 {
		return fmt.Sprintf("[%v, %v] (%d blocks overall)", l[0], l[length-1], length)
	} else {
		return fmt.Sprintf("[%v, ..., %v] (%d blocks overall)", l[0], l[length-1], length)
	}
}
