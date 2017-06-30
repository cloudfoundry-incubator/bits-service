package openstack

import (
	"io"
	"time"

	"github.com/ncw/swift"

	"io/ioutil"

	"bytes"

	"strings"

	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/blobstores/validate"
	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/logger"
	"github.com/pkg/errors"
)

type Blobstore struct {
	containerName         string
	swiftConn             *swift.Connection
	accountMetaTempURLKey string
}

func NewBlobstore(config config.OpenstackBlobstoreConfig) *Blobstore {
	validate.NotEmpty(config.Username)
	validate.NotEmpty(config.ApiKey)
	validate.NotEmpty(config.AuthURL)
	validate.NotEmpty(config.ContainerName)

	swiftConn := &swift.Connection{
		UserName:     config.Username,
		ApiKey:       config.ApiKey,
		AuthUrl:      config.AuthURL,
		AuthVersion:  config.AuthVersion,
		DomainId:     config.DomainId,
		Domain:       config.DomainName,
		Region:       config.Region,
		Internal:     config.Internal,
		EndpointType: swift.EndpointType(config.EndpointType),
	}

	e := swiftConn.Authenticate()
	if e != nil {
		panic(e)
	}
	e = swiftConn.AccountUpdate(map[string]string{"X-Account-Meta-Temp-URL-Key": config.AccountMetaTempURLKey})
	if e != nil {
		panic(e)
	}

	return &Blobstore{
		swiftConn:             swiftConn,
		containerName:         config.ContainerName,
		accountMetaTempURLKey: config.AccountMetaTempURLKey,
	}
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	if !blobstore.containerExists() {
		return false, errors.Errorf("Container not found: '%v'", blobstore.containerName)
	}

	_, _, e := blobstore.swiftConn.Object(blobstore.containerName, path)
	if e != nil {
		if e == swift.ObjectNotFound {
			return false, nil
		}
		return false, errors.Wrapf(e, "Failed to check for %v/%v", blobstore.containerName, path)
	}
	return true, nil
}

func (blobstore *Blobstore) HeadOrRedirectAsGet(path string) (redirectLocation string, err error) {
	return blobstore.swiftConn.ObjectTempUrl(blobstore.containerName, path, blobstore.accountMetaTempURLKey, "GET", time.Now().Add(time.Hour)), nil
}

func (blobstore *Blobstore) containerExists() bool {
	_, _, e := blobstore.swiftConn.Container(blobstore.containerName)
	return e != swift.ContainerNotFound
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	logger.Log.Debugw("Get", "bucket", blobstore.containerName, "path", path)

	if !blobstore.containerExists() {
		return nil, errors.Errorf("Container not found: '%v'", blobstore.containerName)
	}

	buf, e := blobstore.swiftConn.ObjectGetBytes(blobstore.containerName, path)
	if e != nil {
		if e == swift.ObjectNotFound {
			return nil, bitsgo.NewNotFoundError()
		}
		return nil, errors.Wrapf(e, "Container: '%v', path: '%v'", blobstore.containerName, path)
	}
	return ioutil.NopCloser(bytes.NewBuffer(buf)), nil
}

func (blobstore *Blobstore) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	signedUrl, e := blobstore.HeadOrRedirectAsGet(path)
	return nil, signedUrl, e
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	logger.Log.Debugw("Put", "bucket", blobstore.containerName, "path", path)

	if !blobstore.containerExists() {
		return errors.Errorf("Container not found: '%v'", blobstore.containerName)
	}

	_, e := blobstore.swiftConn.ObjectPut(blobstore.containerName, path, src, false, "", "", nil)
	if e != nil {
		return errors.Wrapf(e, "Container: '%v', path: '%v'", blobstore.containerName, path)
	}
	return nil
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	logger.Log.Debugw("Copy", "container", blobstore.containerName, "src", src, "dest", dest)

	if !blobstore.containerExists() {
		return errors.Errorf("Container not found: '%v'", blobstore.containerName)
	}

	_, e := blobstore.swiftConn.ObjectCopy(blobstore.containerName, src, blobstore.containerName, dest, nil)
	if e != nil {
		if e == swift.ObjectNotFound {
			return bitsgo.NewNotFoundError()
		}
		return errors.Wrapf(e, "Container: '%v', src: '%v', dst: '%v'", blobstore.containerName, src, dest)
	}
	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	if !blobstore.containerExists() {
		return errors.Errorf("Container not found: '%v'", blobstore.containerName)
	}

	e := blobstore.swiftConn.ObjectDelete(blobstore.containerName, path)
	if e != nil {
		if e == swift.ObjectNotFound {
			return bitsgo.NewNotFoundError()
		}
		return errors.Wrapf(e, "Container: '%v', path: '%v'", blobstore.containerName, path)
	}
	return nil
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	if !blobstore.containerExists() {
		return errors.Errorf("Container not found: '%v'", blobstore.containerName)
	}

	names, e := blobstore.swiftConn.ObjectNames(blobstore.containerName, &swift.ObjectsOpts{Prefix: prefix})
	if e != nil {
		return errors.Wrapf(e, "Container: '%v', prefix: '%v'", blobstore.containerName, prefix)
	}
	deletionErrs := []error{}
	for _, name := range names {
		e = blobstore.Delete(name)
		if e != nil {
			if _, isNotFoundError := e.(*bitsgo.NotFoundError); !isNotFoundError {
				deletionErrs = append(deletionErrs, e)
			}
		}
	}
	if len(deletionErrs) != 0 {
		return errors.Errorf("Prefix '%v', errors from deleting: %v", prefix, deletionErrs)
	}

	return nil
}

func (blobstore *Blobstore) Sign(resource string, method string, expirationTime time.Time) (signedURL string) {
	if strings.ToLower(method) != "get" && method != "put" {
		panic("The only supported methods are 'put' and 'get'")
	}
	signedURL = blobstore.swiftConn.ObjectTempUrl(blobstore.containerName, resource, blobstore.accountMetaTempURLKey, strings.ToUpper(method), time.Now().Add(time.Hour))
	logger.Log.Debugw("Signed URL", "verb", method, "signed-url", signedURL)
	return
}
