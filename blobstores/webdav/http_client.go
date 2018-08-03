package webdav

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"

	log "github.com/cloudfoundry-incubator/bits-service/logger"
)

func NewHttpClient(pemCerts string, insecureSkipVerify bool) *http.Client {
	log.Log.Infow("Creating Http Client", "insecure-skip-verify", insecureSkipVerify, "ca-cert-path", pemCerts)

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM([]byte(pemCerts))
	if !ok {
		panic("Could not append pemCerts. pemCerts content:\n\n```\n" + pemCerts + "\n```")
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
