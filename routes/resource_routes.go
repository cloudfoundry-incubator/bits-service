package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/basic_auth_middleware"
	"github.com/petergtz/bitsgo/statsd"
	"github.com/urfave/negroni"
)

func SetUpAppStashRoutes(router *mux.Router, blobstore bitsgo.NoRedirectBlobstore) {
	handler := bitsgo.NewAppStashHandler(blobstore)
	router.Path("/app_stash/entries").Methods("POST").HandlerFunc(handler.PostEntries)
	router.Path("/app_stash/matches").Methods("POST").HandlerFunc(handler.PostMatches)
	router.Path("/app_stash/bundles").Methods("POST").HandlerFunc(handler.PostBundles)
}

func SetUpPackageRoutes(router *mux.Router, blobstore bitsgo.Blobstore) {
	setUpDefaultMethodRoutes(
		router.Path("/packages/{identifier}").Subrouter(),
		bitsgo.NewResourceHandler(blobstore, "package", statsd.NewMetricsService()))
}

func SetUpBuildpackRoutes(router *mux.Router, blobstore bitsgo.Blobstore) {
	setUpDefaultMethodRoutes(
		router.Path("/buildpacks/{identifier}").Subrouter(),
		bitsgo.NewResourceHandler(blobstore, "buildpack", statsd.NewMetricsService()))
}

func SetUpDropletRoutes(router *mux.Router, blobstore bitsgo.Blobstore) {
	setUpDefaultMethodRoutes(
		router.Path("/droplets/{identifier:.*}").Subrouter(), // TODO we could probably be more specific in the regex
		bitsgo.NewResourceHandler(blobstore, "droplet", statsd.NewMetricsService()))
}

func SetUpBuildpackCacheRoutes(router *mux.Router, blobstore bitsgo.Blobstore) {
	handler := bitsgo.NewResourceHandler(blobstore, "buildpack_cache", statsd.NewMetricsService())
	router.Path("/buildpack_cache/entries").Methods("DELETE").HandlerFunc(delegateTo(handler.DeleteDir))
	router.Path("/buildpack_cache/entries/{identifier}").Methods("DELETE").HandlerFunc(delegateTo(handler.DeleteDir))
	setUpDefaultMethodRoutes(router.Path("/buildpack_cache/entries/{identifier:.*}").Subrouter(), handler)
}

func setUpDefaultMethodRoutes(router *mux.Router, handler *bitsgo.ResourceHandler) {
	router.Methods("PUT").HandlerFunc(delegateTo(handler.Put))
	router.Methods("HEAD").HandlerFunc(delegateTo(handler.Head))
	router.Methods("GET").HandlerFunc(delegateTo(handler.Get))
	router.Methods("DELETE").HandlerFunc(delegateTo(handler.Delete))
	setRouteNotFoundStatusCode(router, http.StatusMethodNotAllowed)
}

func SetUpSignRoute(router *mux.Router, basicAuthMiddleware *basic_auth_middleware.BasicAuthMiddleware,
	signPackageURLHandler, signDropletURLHandler, signBuildpackURLHandler, signBuildpackCacheURLHandler *bitsgo.SignResourceHandler) {
	router.Path("/sign/packages/{resource}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signPackageURLHandler))
	router.Path("/sign/droplets/{resource:.*}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signDropletURLHandler))
	router.Path("/sign/buildpacks/{resource}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signBuildpackURLHandler))
	router.Path("/sign/{resource:buildpack_cache/entries/.*}").Methods("GET").Handler(wrapWith(basicAuthMiddleware, signBuildpackCacheURLHandler))
}

func wrapWith(basicAuthMiddleware *basic_auth_middleware.BasicAuthMiddleware, handler *bitsgo.SignResourceHandler) http.Handler {
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
