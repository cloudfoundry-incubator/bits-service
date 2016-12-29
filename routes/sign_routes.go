package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo/basic_auth_middleware"
	"github.com/urfave/negroni"
)

type SignURLHandler interface {
	Sign(responseWriter http.ResponseWriter, request *http.Request)
}

func SetUpSignRoute(router *mux.Router, basicAuthMiddleware *basic_auth_middleware.BasicAuthMiddleware,
	signPackageURLHandler, signDropletURLHandler, signBuildpackURLHandler SignURLHandler) {
	router.PathPrefix("/sign/packages").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signPackageURLHandler))
	router.PathPrefix("/sign/droplets").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signDropletURLHandler))
	router.PathPrefix("/sign/buildpacks").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signBuildpackURLHandler))
}

func wrapWith(basicAuthMiddleware *basic_auth_middleware.BasicAuthMiddleware, handler SignURLHandler) http.Handler {
	return negroni.New(
		basicAuthMiddleware,
		negroni.Wrap(http.HandlerFunc(handler.Sign)),
	)
}
