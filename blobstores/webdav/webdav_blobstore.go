package webdav

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"bytes"

	"time"

	bitsgo "github.com/cloudfoundry-incubator/bits-service"
	"github.com/cloudfoundry-incubator/bits-service/config"
	"github.com/cloudfoundry-incubator/bits-service/httputil"
	"github.com/cloudfoundry-incubator/bits-service/logger"
	"github.com/pkg/errors"
)

type Blobstore struct {
	HttpClient            *http.Client
	WebdavPrivateEndpoint string
	WebdavPublicEndpoint  string
	WebdavUsername        string
	WebdavPassword        string
}

func NewBlobstore(c config.WebdavBlobstoreConfig) *Blobstore {
	return &Blobstore{
		WebdavPrivateEndpoint: c.PrivateEndpoint,
		WebdavPublicEndpoint:  c.PublicEndpoint,
		HttpClient:            NewHttpClient(c.CACert(), c.SkipCertVerify),
		WebdavUsername:        c.Username,
		WebdavPassword:        c.Password,
	}
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	url := blobstore.WebdavPrivateEndpoint + "/" + path
	logger.Log.Debugw("Exists", "path", path, "url", url)
	response, e := blobstore.HttpClient.Do(blobstore.newRequestWithBasicAuth("HEAD", url, nil))
	if e != nil {
		return false, errors.Wrapf(e, "Error in Exists, path=%v", path)
	}
	if response.StatusCode == http.StatusOK {
		logger.Log.Debugw("Exists", "result", true)
		return true, nil
	}
	logger.Log.Debugw("Exists", "result", false)
	return false, nil
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	exists, e := blobstore.Exists(path)
	if e != nil {
		return nil, e
	}
	if !exists {
		return nil, bitsgo.NewNotFoundError()
	}

	response, e := blobstore.HttpClient.Get(blobstore.WebdavPrivateEndpoint + "/" + path)

	if e != nil {
		return nil, errors.Wrapf(e, "path=%v", path)
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
		return nil, "", bitsgo.NewNotFoundError()
	}
	// TODO use clock instead
	signedUrl := blobstore.Sign(path, "get", time.Now().Add(1*time.Hour))
	return nil, signedUrl, nil
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	response, e := blobstore.HttpClient.Do(
		blobstore.newRequestWithBasicAuth("PUT", blobstore.WebdavPrivateEndpoint+"/admin/"+path, src))
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
	response, e := blobstore.HttpClient.Do(
		httputil.NewRequest("COPY", blobstore.WebdavPrivateEndpoint+"/admin/"+src, nil).
			WithHeader("Destination", blobstore.WebdavPrivateEndpoint+"/admin/"+dest).
			WithBasicAuth(blobstore.WebdavUsername, blobstore.WebdavPassword).
			Build())
	if e != nil {
		return errors.Wrapf(e, "Request failed. src=%v, dest=%v", src, dest)
	}
	if response.StatusCode == http.StatusNotFound {
		return bitsgo.NewNotFoundError()
	}
	if response.StatusCode < 200 || response.StatusCode > 204 {
		return errors.Errorf("Expected HTTP status code 200-204, but got status code: " + response.Status)
	}
	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	response, e := blobstore.HttpClient.Do(
		blobstore.newRequestWithBasicAuth("DELETE", blobstore.WebdavPrivateEndpoint+"/admin/"+path, nil))
	if e != nil {
		return errors.Wrapf(e, "Request failed. path=%v", path)
	}
	if response.StatusCode < 200 || response.StatusCode > 204 {
		return errors.Errorf("Expected HTTP status code 200-204, but got status code: " + response.Status)
	}
	return nil
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	prefix = appendsSuffixIfNeeded(prefix)
	response, e := blobstore.HttpClient.Do(
		blobstore.newRequestWithBasicAuth("DELETE", blobstore.WebdavPrivateEndpoint+"/admin/"+prefix, nil))
	if e != nil {
		return errors.Wrapf(e, "Request failed. prefix=%v", prefix)
	}

	if response.StatusCode == http.StatusNotFound {
		return bitsgo.NewNotFoundError()
	}

	if response.StatusCode < 200 || response.StatusCode > 204 {
		return errors.Errorf("Expected HTTP status code 200-204, but got status code: " + response.Status)
	}
	return nil
}

func appendsSuffixIfNeeded(prefix string) string {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
}

func (signer *Blobstore) Sign(resource string, method string, expirationTime time.Time) string {
	var url string
	switch strings.ToLower(method) {
	case "put":
		// TODO why do we need a "/" before the resource?
		url = fmt.Sprintf(signer.WebdavPrivateEndpoint+"/sign_for_put?path=/%v&expires=%v", resource, expirationTime.Unix())
	case "get":
		url = fmt.Sprintf(signer.WebdavPrivateEndpoint+"/sign?path=/%v&expires=%v", resource, expirationTime.Unix())
	}
	response, e := signer.HttpClient.Do(
		httputil.NewRequest("GET", url, nil).
			WithBasicAuth(signer.WebdavUsername, signer.WebdavPassword).
			Build())
	if e != nil {
		return "Error during signing. Error: " + e.Error()
	}
	if response.StatusCode != http.StatusOK {
		return "Error during signing. Error code: " + response.Status
	}
	defer response.Body.Close()
	content, e := ioutil.ReadAll(response.Body)
	if e != nil {
		return "Error reading response body. Error: " + e.Error()
	}

	signedUrl := httputil.MustParse(string(content))

	// TODO Is this really what we want to do?
	signedUrl.Host = httputil.MustParse(signer.WebdavPublicEndpoint).Host

	// TODO: in the legacy bits-service, this is hard-coded to http, although it should probably be:
	//       httputil.MustParse(signer.WebdavPublicEndpoint).Scheme
	//       However, certificates currently do not work out yet, when Stager (Rep) tries to access the URL.
	//       It will then error, because it cannot verify the certificate. Hence, keeping the hard-coded value
	//       for now to be functinally equivalent.
	signedUrl.Scheme = "http"

	return signedUrl.String()
}

func (blobstore *Blobstore) newRequestWithBasicAuth(method string, urlStr string, body io.Reader) *http.Request {
	logger.Log.Debugw("Building HTTP request", "method", method, "url", urlStr, "has-body", body != nil, "user", blobstore.WebdavUsername)
	return httputil.NewRequest(method, urlStr, body).
		WithBasicAuth(blobstore.WebdavUsername, blobstore.WebdavPassword).
		Build()
}
