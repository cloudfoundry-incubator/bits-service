package webdav

import (
	"net/http"
	"strings"

	"fmt"
	"time"

	"io/ioutil"

	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/httputil"
)

func NewWebdavResourceSigner(config config.WebdavBlobstoreConfig) *WebdavResourceSigner {
	return &WebdavResourceSigner{
		webdavPrivateEndpoint: config.PrivateEndpoint,
		webdavPublicEndpoint:  config.PublicEndpoint,
		httpClient:            NewHttpClient(config.CACert(), config.SkipCertVerify),
		webdavUsername:        config.Username,
		webdavPassword:        config.Password,
	}
}

type WebdavResourceSigner struct {
	httpClient            *http.Client
	webdavPrivateEndpoint string
	webdavPublicEndpoint  string
	webdavUsername        string
	webdavPassword        string
}

func (signer *WebdavResourceSigner) Sign(resource string, method string) string {
	var url string
	switch strings.ToLower(method) {
	case "put":
		// TODO why do we need a "/" before the resource?
		url = fmt.Sprintf(signer.webdavPrivateEndpoint+"/sign_for_put?path=/%v&expires=%v", resource, time.Now().Unix()+3600)
	case "get":
		url = fmt.Sprintf(signer.webdavPrivateEndpoint+"/sign?path=/%v&expires=%v", resource, time.Now().Unix()+3600)
	}
	request, _ := http.NewRequest("GET", url, nil)
	request.SetBasicAuth(signer.webdavUsername, signer.webdavPassword)
	response, e := signer.httpClient.Do(request)
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
	signedUrl.Host = httputil.MustParse(signer.webdavPublicEndpoint).Host
	signedUrl.Scheme = httputil.MustParse(signer.webdavPublicEndpoint).Scheme

	return signedUrl.String()
}
