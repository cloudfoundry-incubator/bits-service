package bitsgo

import (
	"fmt"
	"net/http"
)

type ResourceSigner interface {
	Sign(resource string, method string) (signedURL string)
}

type SignResourceHandler struct {
	signer ResourceSigner
}

func NewSignResourceHandler(signer ResourceSigner) *SignResourceHandler {
	return &SignResourceHandler{signer}
}

func (handler *SignResourceHandler) Sign(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	method := request.URL.Query().Get("verb")
	if method == "" {
		method = "get"
	}
	fmt.Fprint(responseWriter, handler.signer.Sign(params["resource"], method))
}
