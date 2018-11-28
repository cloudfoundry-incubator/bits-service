package acceptance

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func StartServer(configYamlFile string) (session *gexec.Session) {
	pathToWebserver, err := gexec.Build("github.com/cloudfoundry-incubator/bits-service/cmd/bitsgo")
	Ω(err).ShouldNot(HaveOccurred())

	os.Setenv("BITS_LISTEN_ADDR", "127.0.0.1")
	session, err = gexec.Start(exec.Command(pathToWebserver, "--config", configYamlFile), GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())
	time.Sleep(200 * time.Millisecond)
	Expect(session.ExitCode()).To(Equal(-1), "Webserver error message: %s", string(session.Err.Contents()))
	return
}

func CreateTLSClient(caCertFile string) *http.Client {
	caCert, err := ioutil.ReadFile(caCertFile)
	Ω(err).ShouldNot(HaveOccurred())
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs: caCertPool,
	}}}
}
