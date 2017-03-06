package webdav

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"

	log "github.com/petergtz/bitsgo/logger"
	"github.com/uber-go/zap"
)

func NewHttpClient(pemCerts string, insecureSkipVerify bool) *http.Client {
	log.Log.Info("Creating Http Client",
		zap.Bool("insecure-skip-verify", insecureSkipVerify),
		zap.String("ca-cert-path", pemCerts))

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
