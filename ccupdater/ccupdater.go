package ccupdater

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/petergtz/bitsgo/logger"

	"github.com/petergtz/bitsgo"

	"github.com/pkg/errors"
)

type CCUpdater struct {
	httpClient HttpClient
	endpoint   string
	method     string
}

type processingUploadPayload struct {
	State string `json:"state"`
}

type checksum struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type successPayload struct {
	State     string     `json:"state"`
	Checksums []checksum `json:"checksums"`
}

type failurePayload struct {
	State string `json:"state"`
	Error string `json:"error"`
}

type HttpClient interface {
	Do(*http.Request) (*http.Response, error)
}

func NewCCUpdater(endpoint string, method string, clientCertFile string, clientKeyFile string, caCertFile string) *CCUpdater {
	u, e := url.Parse(endpoint)
	if e != nil {
		logger.Log.Fatalw("Could not parse endpoint", "endpoint", endpoint, "error", e)
	}

	var tlsConfig *tls.Config
	if u.Scheme == "https" {
		tlsConfig = loadTLSConfig(clientCertFile, clientKeyFile, caCertFile)
	}
	return NewCCUpdaterWithHttpClient(endpoint, method, &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	})
}

func NewCCUpdaterWithHttpClient(endpoint string, method string, httpClient HttpClient) *CCUpdater {
	return &CCUpdater{
		httpClient: httpClient,
		endpoint:   endpoint,
		method:     method,
	}
}

func loadTLSConfig(clientCertFile string, clientKeyFile string, caCertFile string) *tls.Config {
	cert, e := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if e != nil {
		logger.Log.Fatalw("Could not load X509 key pair", "error", e, "client-cert-file", clientCertFile, "client-key-file", clientKeyFile)
	}

	caCert, e := ioutil.ReadFile(caCertFile)
	if e != nil {
		logger.Log.Fatalw("Could not read CA Cert file", "error", e, "ca-cert-file", caCertFile)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()
	return tlsConfig
}

func (updater *CCUpdater) NotifyProcessingUpload(guid string) error {
	return updater.update(guid, processingUploadPayload{"PROCESSING_UPLOAD"})
}

func (updater *CCUpdater) NotifyUploadSucceeded(guid string, sha1 string, sha256 string) error {
	return updater.update(guid, successPayload{
		"READY",
		[]checksum{
			checksum{Type: "sha1", Value: sha1},
			checksum{Type: "sha256", Value: sha256},
		},
	})
}

func (updater *CCUpdater) NotifyUploadFailed(guid string, e error) error {
	return updater.update(guid, failurePayload{"FAILED", e.Error()})
}

func (updater *CCUpdater) update(guid string, p interface{}) error {
	payload, e := json.Marshal(p)
	if e != nil {
		logger.Log.Fatalw("Unexpected error in CC Updater update when marshalling payload",
			"error", e, "guid", guid, "payload", p)
	}

	r, e := http.NewRequest(updater.method, updater.endpoint+"/"+guid, bytes.NewReader(payload))
	if e != nil {
		logger.Log.Fatalw("Unexpected error in CC Updater update when creating new request",
			"error", e, "guid", guid, "payload", p)
	}
	resp, e := updater.httpClient.Do(r)
	if e != nil {
		return errors.Wrapf(e, "Could not make request against CC (GUID: \"%v\")", guid)
	}
	if resp.StatusCode == http.StatusNotFound {
		return bitsgo.NewNotFoundError()
	}
	if resp.StatusCode == http.StatusUnprocessableEntity {
		return bitsgo.NewStateForbiddenError()
	}
	return nil
}
