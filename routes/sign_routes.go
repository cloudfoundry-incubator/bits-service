package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo/basic_auth_middleware"
	"github.com/urfave/negroni"
)

func SetUpSignRoute(router *mux.Router, basicAuthMiddleware *basic_auth_middleware.BasicAuthMiddleware,
	signPackageURLHandler, signDropletURLHandler, signBuildpackURLHandler, signBuildpackCacheURLHandler *SignResourceHandler) {
	router.Path("/sign/packages/{resource}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signPackageURLHandler))
	router.Path("/sign/droplets/{resource:.*}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signDropletURLHandler))
	router.Path("/sign/buildpacks/{resource}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signBuildpackURLHandler))
	router.Path("/sign/{resource:buildpack_cache/entries/.*}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signBuildpackCacheURLHandler))
}

func wrapWith(basicAuthMiddleware *basic_auth_middleware.BasicAuthMiddleware, handler *SignResourceHandler) http.Handler {
	return negroni.New(
		basicAuthMiddleware,
		negroni.Wrap(http.HandlerFunc(handler.Sign)),
	)
}
