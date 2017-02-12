package webdav

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"
)

func NewHttpClient(pemCerts string, insecureSkipVerify bool) *http.Client {
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM([]byte(pemCerts))
	if !ok {
		panic("Could not append pemCerts. pemCerts content:\n\n```\n" + pemCerts + "```")
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureSkipVerify,
				RootCAs:            caCertPool,
			},
		},
		Timeout: 15 * time.Minute,
	}
}
