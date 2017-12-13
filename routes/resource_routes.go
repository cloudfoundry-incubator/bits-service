package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/middlewares"
	"github.com/urfave/negroni"
)

func SetUpAppStashRoutes(router *mux.Router, appStashHandler *bitsgo.AppStashHandler) {
	router.Path("/app_stash/entries").Methods("POST").HandlerFunc(appStashHandler.PostEntries)
	router.Path("/app_stash/matches").Methods("POST").HandlerFunc(appStashHandler.PostMatches)
	router.Path("/app_stash/bundles").Methods("POST").HandlerFunc(appStashHandler.PostBundles)
}

func SetUpPackageRoutes(router *mux.Router, resourceHandler *bitsgo.ResourceHandler) {
	setUpDefaultMethodRoutes(router.Path("/packages/{identifier}").Subrouter(), resourceHandler)
}

func SetUpBuildpackRoutes(router *mux.Router, resourceHandler *bitsgo.ResourceHandler) {
	setUpDefaultMethodRoutes(router.Path("/buildpacks/{identifier}").Subrouter(), resourceHandler)
}

func SetUpDropletRoutes(router *mux.Router, resourceHandler *bitsgo.ResourceHandler) {
	setUpDefaultMethodRoutes(
		router.Path("/droplets/{identifier:.*}").Subrouter(), // TODO we could probably be more specific in the regex
		resourceHandler)
}

func SetUpBuildpackCacheRoutes(router *mux.Router, resourceHandler *bitsgo.ResourceHandler) {
	router.Path("/buildpack_cache/entries").Methods("DELETE").HandlerFunc(delegateTo(resourceHandler.DeleteDir))
	router.Path("/buildpack_cache/entries/{identifier}").Methods("DELETE").HandlerFunc(delegateTo(resourceHandler.DeleteDir))
	setUpDefaultMethodRoutes(router.Path("/buildpack_cache/entries/{identifier:.*}").Subrouter(), resourceHandler)
}

func setUpDefaultMethodRoutes(router *mux.Router, handler *bitsgo.ResourceHandler) {
	router.Methods("PUT").HandlerFunc(delegateTo(handler.AddOrReplace))
	router.Methods("HEAD").HandlerFunc(delegateTo(handler.HeadOrRedirectAsGet))
	router.Methods("GET").HandlerFunc(delegateTo(handler.Get))
	router.Methods("DELETE").HandlerFunc(delegateTo(handler.Delete))
	setRouteNotFoundStatusCode(router, http.StatusMethodNotAllowed)
}

func SetUpSignRoute(router *mux.Router, basicAuthMiddleware *middlewares.BasicAuthMiddleware,
	signPackageURLHandler, signDropletURLHandler, signBuildpackURLHandler, signBuildpackCacheURLHandler *bitsgo.SignResourceHandler) {
	router.Path("/sign/packages/{resource}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signPackageURLHandler))
	router.Path("/sign/droplets/{resource:.*}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signDropletURLHandler))
	router.Path("/sign/buildpacks/{resource}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signBuildpackURLHandler))
	router.Path("/sign/{resource:buildpack_cache/entries/.*}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signBuildpackCacheURLHandler))
}

func wrapWith(basicAuthMiddleware *middlewares.BasicAuthMiddleware, handler *bitsgo.SignResourceHandler) http.Handler {
	return negroni.New(
		basicAuthMiddleware,
		negroni.Wrap(http.HandlerFunc(delegateTo(handler.Sign))),
	)
}

func setRouteNotFoundStatusCode(router *mux.Router, statusCode int) {
	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	})
}

func delegateTo(delegate func(http.ResponseWriter, *http.Request, map[string]string)) func(http.ResponseWriter, *http.Request) {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		delegate(responseWriter, request, mux.Vars(request))
	}
}
