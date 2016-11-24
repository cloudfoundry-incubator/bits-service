package routes

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"io/ioutil"

	"github.com/gorilla/mux"
)

type NotFoundError struct {
	error
}

func NewNotFoundError() *NotFoundError {
	return &NotFoundError{fmt.Errorf("NotFoundError")}
}

type Blobstore interface {
	Get(path string) (body io.ReadCloser, redirectLocation string, err error)
	Put(path string, src io.ReadSeeker) (redirectLocation string, err error)
	Exists(path string) (bool, error)
}

func SetUpAppStashRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &AppStashHandler{blobstore: blobstore}
	router.Path("/app_stash/entries").Methods("POST").HandlerFunc(handler.PostEntries)
	router.Path("/app_stash/matches").Methods("POST").HandlerFunc(handler.PostMatches)
	router.Path("/app_stash/bundles").Methods("POST").HandlerFunc(handler.PostBundles)
}

func SetUpPackageRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &ResourceHandler{blobstore: blobstore, resourceType: "package"}
	router.Path("/packages/{guid}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/packages/{guid}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/packages/{guid}").Methods("DELETE").HandlerFunc(handler.Delete)
}

func SetUpBuildpackRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &ResourceHandler{blobstore: blobstore, resourceType: "buildpack"}
	router.Path("/buildpacks/{guid}").Methods("PUT").HandlerFunc(handler.Put)
	// TODO change Put/Get/etc. signature to allow this:
	// router.Path("/buildpacks/{guid}").Methods("PUT").HandlerFunc(delegateTo(handler.Put))
	router.Path("/buildpacks/{guid}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/buildpacks/{guid}").Methods("DELETE").HandlerFunc(handler.Delete)
}

func delegateTo(delegate func(http.ResponseWriter, *http.Request, map[string]string)) func(http.ResponseWriter, *http.Request) {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		delegate(responseWriter, request, mux.Vars(request))
	}
}

func SetUpDropletRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &ResourceHandler{blobstore: blobstore, resourceType: "droplet"}
	router.Path("/droplets/{guid}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/droplets/{guid}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/droplets/{guid}").Methods("DELETE").HandlerFunc(handler.Delete)
}

func SetUpBuildpackCacheRoutes(router *mux.Router, blobstore Blobstore) {
	handler := &BuildpackCacheHandler{blobStore: blobstore}
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("PUT").HandlerFunc(handler.Put)
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("GET").HandlerFunc(handler.Get)
	router.Path("/buildpack_cache/entries/{app_guid}/{stack_name}").Methods("DELETE").HandlerFunc(handler.Delete)
	router.Path("/buildpack_cache/entries/{app_guid}/").Methods("DELETE").HandlerFunc(handler.DeleteAppGuid)
	router.Path("/buildpack_cache/entries").Methods("DELETE").HandlerFunc(handler.DeleteEntries)
}

type AppStashHandler struct {
	blobstore Blobstore
}

func (handler *AppStashHandler) PostEntries(responseWriter http.ResponseWriter, request *http.Request) {
	uploadedFile, _, e := request.FormFile("application")
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(responseWriter, "Could not retrieve 'application' form parameter")
		return
	}
	defer uploadedFile.Close()

	tempZipFile, e := ioutil.TempFile("", "")
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempZipFile.Name())
	defer tempZipFile.Close()

	_, e = io.Copy(tempZipFile, uploadedFile)
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	openZipFile, e := zip.OpenReader(tempZipFile.Name())
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(responseWriter, "Not a valid zip file")
		return
	}
	defer openZipFile.Close()

	for _, zipFileEntry := range openZipFile.File {
		e = copyTo(handler.blobstore, zipFileEntry, responseWriter)
		if e != nil {
			log.Printf("Cannot copy file (%v) to blobstore. Caused by: %v", zipFileEntry.Name, e)
			responseWriter.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func copyTo(blobstore Blobstore, zipFileEntry *zip.File, responseWriter http.ResponseWriter) error {
	unzippedReader, e := zipFileEntry.Open()
	if e != nil {
		return e
	}
	defer unzippedReader.Close()

	tempZipEntryFile, e := ioutil.TempFile("", zipFileEntry.Name)
	if e != nil {
		return e
	}
	defer os.Remove(tempZipEntryFile.Name())
	defer tempZipEntryFile.Close()

	sha, e := writeToFile(tempZipEntryFile, unzippedReader)
	if e != nil {
		return e
	}

	entryFileRead, e := os.Open(tempZipEntryFile.Name())
	if e != nil {
		return e
	}
	defer entryFileRead.Close()

	// TODO: this assumes no redirect on PUTs. Is that always true?
	_, e = blobstore.Put(sha, entryFileRead)
	if e != nil {
		return e
	}

	return nil
}

func writeToFile(writer io.Writer, reader io.Reader) (sha string, e error) {
	checkSum := sha1.New()
	multiWriter := io.MultiWriter(writer, checkSum)

	_, e = io.Copy(multiWriter, reader)
	if e != nil {
		return "", fmt.Errorf("error copying. Caused by: %v", e)
	}

	return string(checkSum.Sum(nil)), nil
}

func (handler *AppStashHandler) PostMatches(responseWriter http.ResponseWriter, request *http.Request) {
	body, e := ioutil.ReadAll(request.Body)
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	var sha1s []map[string]string
	e = json.Unmarshal(body, &sha1s)
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(responseWriter, "Invalid body %v", body)
		return
	}
	var responseSha1 []map[string]string
	for _, entry := range sha1s {
		exists, e := handler.blobstore.Exists(entry["sha1"])
		if e != nil {
			log.Println(e)
			responseWriter.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !exists {
			responseSha1 = append(responseSha1, map[string]string{"sha1": entry["sha1"]})
		}
	}
	response, e := json.Marshal(&responseSha1)
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(responseWriter, "%v", response)
}

func (handler *AppStashHandler) PostBundles(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO
}

type ResourceHandler struct {
	blobstore    Blobstore
	resourceType string
}

func (handler *ResourceHandler) Put(responseWriter http.ResponseWriter, request *http.Request) {
	file, _, e := request.FormFile(handler.resourceType)
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(responseWriter, "Could not retrieve '%s' form parameter", handler.resourceType)
		return
	}
	defer file.Close()

	redirectLocation, e := handler.blobstore.Put(pathFor(mux.Vars(request)["guid"]), file)

	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	if redirectLocation != "" {
		responseWriter.WriteHeader(http.StatusFound)
		responseWriter.Header().Set("Location", redirectLocation)
		return
	}

	responseWriter.WriteHeader(http.StatusCreated)
}

func (handler *ResourceHandler) Get(responseWriter http.ResponseWriter, request *http.Request) {
	body, redirectLocation, e := handler.blobstore.Get(pathFor(mux.Vars(request)["guid"]))
	switch e.(type) {
	case *NotFoundError:
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	case error:
		log.Println(e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	if redirectLocation != "" {
		responseWriter.WriteHeader(http.StatusFound)
		responseWriter.Header().Set("Location", redirectLocation)
		return
	}
	defer body.Close()
	responseWriter.WriteHeader(http.StatusOK)
	io.Copy(responseWriter, body)
}

func (handler *ResourceHandler) Delete(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO
}

func pathFor(identifier string) string {
	return fmt.Sprintf("/%s/%s/%s", identifier[0:2], identifier[2:4], identifier)
}

type BuildpackCacheHandler struct {
	blobStore Blobstore
}

func (handler *BuildpackCacheHandler) Put(responseWriter http.ResponseWriter, request *http.Request) {
	file, _, e := request.FormFile("buildpack_cache")
	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(responseWriter, "Could not retrieve buildpack_cache form parameter")
		return
	}
	defer file.Close()

	redirecLocation, e := handler.blobStore.Put(
		fmt.Sprintf("/buildpack_cache/entries/%s/%s", mux.Vars(request)["app_guid"], mux.Vars(request)["stack_name"]), file)

	if e != nil {
		log.Println(e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	if redirecLocation != "" {
		responseWriter.WriteHeader(http.StatusFound)
		responseWriter.Header().Set("Location", redirecLocation)
		return
	}
	responseWriter.WriteHeader(http.StatusCreated)
}

func (handler *BuildpackCacheHandler) Get(responseWriter http.ResponseWriter, request *http.Request) {
	body, redirectLocation, e := handler.blobStore.Get(
		fmt.Sprintf("/buildpack_cache/entries/%s/%s", mux.Vars(request)["app_guid"], mux.Vars(request)["stack_name"]))
	switch e.(type) {
	case NotFoundError:
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	case error:
		log.Println(e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	if redirectLocation != "" {
		responseWriter.WriteHeader(http.StatusFound)
		responseWriter.Header().Set("Location", redirectLocation)
		return
	}
	defer body.Close()
	responseWriter.WriteHeader(http.StatusOK)
	io.Copy(responseWriter, body)
}

func (handler *BuildpackCacheHandler) Delete(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO
}

func (handler *BuildpackCacheHandler) DeleteAppGuid(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO
}

func (handler *BuildpackCacheHandler) DeleteEntries(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO
}
