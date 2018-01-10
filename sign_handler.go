package bitsgo

import (
	"fmt"
	"net/http"
	"time"

	"github.com/benbjohnson/clock"
)

type ResourceSigner interface {
	Sign(resource string, method string, expirationTime time.Time) (signedURL string)
}

type SignResourceHandler struct {
	clock                                clock.Clock
	putResourceSigner, getResourceSigner ResourceSigner
}

func NewSignResourceHandler(getResourceSigner, putResourceSigner ResourceSigner) *SignResourceHandler {
	return &SignResourceHandler{
		getResourceSigner: getResourceSigner,
		putResourceSigner: putResourceSigner,
		clock:             clock.New(),
	}
}

func (handler *SignResourceHandler) Sign(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	method := params["verb"]
	var signer ResourceSigner

	if method == "" {
		method = "get"
	}

	switch method {
	case "get":
		signer = handler.getResourceSigner
	case "put":
		signer = handler.putResourceSigner
	default:
		responseWriter.WriteHeader(http.StatusBadRequest)
		responseWriter.Write([]byte("Invalid verb: " + method))
		return
	}

	signature := signer.Sign(params["resource"], method, handler.clock.Now().Add(1*time.Hour))
	fmt.Fprint(responseWriter, signature)
}
