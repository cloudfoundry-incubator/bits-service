package routes

import (
	"net/http"

	"github.com/gorilla/mux"
)

type SignURLHandler interface {
	Sign(responseWriter http.ResponseWriter, request *http.Request)
}

func SetUpSignRoute(router *mux.Router,
	signPackageURLHandler, signDropletURLHandler, signBuildpackURLHandler SignURLHandler) {
	router.PathPrefix("/sign/packages").Methods("GET").HandlerFunc(signPackageURLHandler.Sign)
	router.PathPrefix("/sign/droplets").Methods("GET").HandlerFunc(signDropletURLHandler.Sign)
	router.PathPrefix("/sign/buildpacks").Methods("GET").HandlerFunc(signBuildpackURLHandler.Sign)
}
