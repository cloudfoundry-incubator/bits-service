package routes

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
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

func (handler *SignResourceHandler) Sign(responseWriter http.ResponseWriter, request *http.Request) {
	method := request.URL.Query().Get("verb")
	if method == "" {
		method = "get"
	}
	fmt.Fprint(responseWriter, handler.signer.Sign(mux.Vars(request)["resource"], method))
}
