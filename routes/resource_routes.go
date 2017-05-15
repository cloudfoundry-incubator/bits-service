package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	. "github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/statsd"
)

func SetUpAppStashRoutes(router *mux.Router, blobstore NoRedirectBlobstore) {
	handler := NewAppStashHandler(blobstore)
	router.Path("/app_stash/entries").Methods("POST").HandlerFunc(handler.PostEntries)
	router.Path("/app_stash/matches").Methods("POST").HandlerFunc(handler.PostMatches)
	router.Path("/app_stash/bundles").Methods("POST").HandlerFunc(handler.PostBundles)
}

func SetUpPackageRoutes(router *mux.Router, blobstore Blobstore) {
	setUpDefaultMethodRoutes(
		router.Path("/packages/{identifier}").Subrouter(),
		NewResourceHandler(blobstore, "package", statsd.NewMetricsService()))
}

func SetUpBuildpackRoutes(router *mux.Router, blobstore Blobstore) {
	setUpDefaultMethodRoutes(
		router.Path("/buildpacks/{identifier}").Subrouter(),
		NewResourceHandler(blobstore, "buildpack", statsd.NewMetricsService()))
}

func SetUpDropletRoutes(router *mux.Router, blobstore Blobstore) {
	setUpDefaultMethodRoutes(
		router.Path("/droplets/{identifier:.*}").Subrouter(), // TODO we could probably be more specific in the regex
		NewResourceHandler(blobstore, "droplet", statsd.NewMetricsService()))
}

func SetUpBuildpackCacheRoutes(router *mux.Router, blobstore Blobstore) {
	handler := NewResourceHandler(blobstore, "buildpack_cache", statsd.NewMetricsService())
	router.Path("/buildpack_cache/entries").Methods("DELETE").HandlerFunc(handler.DeleteDir)
	router.Path("/buildpack_cache/entries/{identifier}").Methods("DELETE").HandlerFunc(handler.DeleteDir)
	setUpDefaultMethodRoutes(router.Path("/buildpack_cache/entries/{identifier:.*}").Subrouter(), handler)
}

func setUpDefaultMethodRoutes(router *mux.Router, handler *ResourceHandler) {
	// TODO Should we change Put/Get/etc. signature to allow this:
	// router.Methods("PUT").HandlerFunc(delegateTo(handler.Put))
	router.Methods("PUT").HandlerFunc(handler.Put)
	router.Methods("HEAD").HandlerFunc(handler.Head)
	router.Methods("GET").HandlerFunc(handler.Get)
	router.Methods("DELETE").HandlerFunc(handler.Delete)
	setRouteNotFoundStatusCode(router, http.StatusMethodNotAllowed)
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
