package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()
	packageHandler := &PackageHandler{name: "TestServer"}
	internalHostName := "ups.127.0.0.1.xip.io"

	internalRouter := router.Host(internalHostName).Subrouter()

	internalRouter.Path("/packages/{guid}").Methods("PUT").HandlerFunc(packageHandler.put)
	internalRouter.Path("/packages/{guid}").Methods("GET").HandlerFunc(packageHandler.get)
	internalRouter.Path("/packages/{guid}").Methods("DELETE").HandlerFunc(packageHandler.delete)

	srv := &http.Server{
		Handler:      router,
		Addr:         "0.0.0.0:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

type DataStore interface {
}

type PackageHandler struct {
	name string
}

func (handler *PackageHandler) put(responseWriter http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(responseWriter, "%s says: Hello, %s\n\n", handler.name, mux.Vars(request)["guid"])
}

func (handler *PackageHandler) get(responseWriter http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(responseWriter, "%s says: Hello, %s\n\n", handler.name, mux.Vars(request)["guid"])
}

func (handler *PackageHandler) delete(responseWriter http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(responseWriter, "%s says: Hello, %s\n\n", handler.name, mux.Vars(request)["guid"])
}
