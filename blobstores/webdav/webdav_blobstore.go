package webdav

import (
	"io"
	"net/http"

	"bytes"

	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/httputil"
	"github.com/petergtz/bitsgo/logger"
	"github.com/petergtz/bitsgo/routes"
	"github.com/pkg/errors"
	"github.com/uber-go/zap"
)

type Blobstore struct {
	httpClient            *http.Client
	webdavPrivateEndpoint string
	signer                *WebdavResourceSigner
	webdavUsername        string
	webdavPassword        string
}

func NewBlobstore(c config.WebdavBlobstoreConfig) *Blobstore {
	return &Blobstore{
		webdavPrivateEndpoint: c.PrivateEndpoint,
		httpClient:            NewHttpClient(c.CACert(), c.SkipCertVerify),
		signer:                NewWebdavResourceSigner(c),
		webdavUsername:        c.Username,
		webdavPassword:        c.Password,
	}
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	url := blobstore.webdavPrivateEndpoint + "/" + path
	logger.Log.Debug("Exists", zap.String("path", path), zap.String("url", url))
	response, e := blobstore.httpClient.Do(blobstore.newRequestWithBasicAuth("HEAD", url, nil))
	if e != nil {
		return false, errors.Wrapf(e, "Error in Exists, path=%v", path)
	}
	if response.StatusCode == http.StatusOK {
		logger.Log.Debug("Exists", zap.Bool("result", true))
		return true, nil
	}
	logger.Log.Debug("Exists", zap.Bool("result", false))
	return false, nil
}

func (blobstore *Blobstore) HeadOrRedirectAsGet(path string) (redirectLocation string, err error) {
	_, redirectLocation, e := blobstore.GetOrRedirect(path)
	return redirectLocation, e
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	exists, e := blobstore.Exists(path)
	if e != nil {
		return nil, e
	}
	if !exists {
		return nil, routes.NewNotFoundError()
	}

	response, e := blobstore.httpClient.Get(blobstore.webdavPrivateEndpoint + "/" + path)

	if e != nil {
		return nil, errors.Wrapf(e, "path=%v")
	}
	if response.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Unexpected status code %v. Expected status OK", response.Status)
	}

	return response.Body, nil
}

func (blobstore *Blobstore) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	exists, e := blobstore.Exists(path)
	if e != nil {
		return nil, "", e
	}
	if !exists {
		return nil, "", routes.NewNotFoundError()
	}
	signedUrl := blobstore.signer.Sign(path, "get")
	return nil, signedUrl, nil
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	response, e := blobstore.httpClient.Do(
		blobstore.newRequestWithBasicAuth("PUT", blobstore.webdavPrivateEndpoint+"/admin/"+path, src))
	if e != nil {
		return errors.Wrapf(e, "Request failed. path=%v", path)
	}
	if response.StatusCode < 200 || response.StatusCode > 204 {
		return errors.Errorf("Expected StatusCreated, but got status code: " + response.Status)
	}
	return nil
}

func (blobstore *Blobstore) PutOrRedirect(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	return "", blobstore.Put(path, src)
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	_, e := blobstore.PutOrRedirect(dest, bytes.NewReader(nil))
	if e != nil {
		return e
	}
	response, e := blobstore.httpClient.Do(
		httputil.NewRequest("COPY", blobstore.webdavPrivateEndpoint+"/admin/"+src, nil).
			WithHeader("Destination", blobstore.webdavPrivateEndpoint+"/admin/"+dest).
			WithBasicAuth(blobstore.webdavUsername, blobstore.webdavPassword).
			Build())
	if e != nil {
		return errors.Wrapf(e, "Request failed. src=%v, dest=%v", src, dest)
	}
	if response.StatusCode == http.StatusNotFound {
		return routes.NewNotFoundError()
	}
	if response.StatusCode < 200 || response.StatusCode > 204 {
		return errors.Errorf("Expected StatusCreated, but got status code: " + response.Status)
	}
	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	response, e := blobstore.httpClient.Do(
		blobstore.newRequestWithBasicAuth("DELETE", blobstore.webdavPrivateEndpoint+"/admin/"+path, nil))
	if e != nil {
		return errors.Wrapf(e, "Request failed. path=%v", path)
	}
	if response.StatusCode < 200 || response.StatusCode > 204 {
		return errors.Errorf("Expected StatusCreated, but got status code: " + response.Status)
	}
	return nil
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	if prefix != "" {
		prefix += "/"
	}
	response, e := blobstore.httpClient.Do(
		blobstore.newRequestWithBasicAuth("DELETE", blobstore.webdavPrivateEndpoint+"/admin/"+prefix, nil))
	if e != nil {
		return errors.Wrapf(e, "Request failed. prefix=%v", prefix)
	}

	if response.StatusCode == http.StatusNotFound {
		return routes.NewNotFoundError()
	}

	if response.StatusCode < 200 || response.StatusCode > 204 {
		return errors.Errorf("Expected StatusCreated, but got status code: " + response.Status)
	}
	return nil
}

func (blobstore *Blobstore) newRequestWithBasicAuth(method string, urlStr string, body io.Reader) *http.Request {
	return httputil.NewRequest(method, urlStr, body).
		WithBasicAuth(blobstore.webdavUsername, blobstore.webdavPassword).
		Build()
}
