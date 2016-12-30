package routes

import (
	"net/http"

	"github.com/gorilla/mux"
)

func SetUpAppStashRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &AppStashHandler{blobstore: blobstore}
	router.Path("/app_stash/entries").Methods("POST").HandlerFunc(handler.PostEntries)
	router.Path("/app_stash/matches").Methods("POST").HandlerFunc(handler.PostMatches)
	router.Path("/app_stash/bundles").Methods("POST").HandlerFunc(handler.PostBundles)
}

func SetUpPackageRoutes(router *mux.Router, blobstore Blobstore) {
	setUpDefaultMethodRoutes(
		router.Path("/packages/{guid}").Subrouter(),
		&ResourceHandler{blobstore: blobstore, resourceType: "package"})
}

func SetUpBuildpackRoutes(router *mux.Router, blobstore Blobstore) {
	setUpDefaultMethodRoutes(
		router.Path("/buildpacks/{guid}").Subrouter(),
		&ResourceHandler{blobstore: blobstore, resourceType: "buildpack"})
}

func SetUpDropletRoutes(router *mux.Router, blobstore Blobstore) {
	setUpDefaultMethodRoutes(
		router.Path("/droplets/{guid:.*}").Subrouter(), // TODO we could probably be more specific in the regex
		&ResourceHandler{blobstore: blobstore, resourceType: "droplet"})
}

func SetUpBuildpackCacheRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &BuildpackCacheHandler{blobStore: blobstore}
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("HEAD").HandlerFunc(handler.Head)
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("DELETE").HandlerFunc(handler.Delete)
	router.Path("/buildpack_cache/entries/{app_guid}").Methods("DELETE").HandlerFunc(handler.DeleteAppGuid)
	router.Path("/buildpack_cache/entries").Methods("DELETE").HandlerFunc(handler.DeleteEntries)
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
