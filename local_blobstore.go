package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type LocalBlobStore struct {
	pathPrefix string
}

func (blobstore *LocalBlobStore) Exists(path string) bool {
	_, err := os.Stat(filepath.Join(blobstore.pathPrefix, path))
	return err == nil
}
func (blobstore *LocalBlobStore) Get(path string, responseWriter http.ResponseWriter) {
	file, e := os.Open(filepath.Join(blobstore.pathPrefix, path))

	if os.IsNotExist(e) {
		responseWriter.WriteHeader(404)
		return
	}
	if e != nil {
		responseWriter.WriteHeader(500)
		log.Printf("Error while opening file %v. Caused by: %v", path, e)
		return
	}
	io.Copy(responseWriter, file)
}

func (blobstore *LocalBlobStore) Put(path string, src io.Reader, responseWriter http.ResponseWriter) {
	e := os.MkdirAll(filepath.Dir(filepath.Join(blobstore.pathPrefix, path)), os.ModeDir|0755)
	if e != nil {
		log.Printf("Error while creating directories for %v. Caused by: %v", path, e)
		responseWriter.WriteHeader(500)
		return
	}
	file, e := os.Create(filepath.Join(blobstore.pathPrefix, path))
	defer file.Close()
	if e != nil {
		log.Printf("Error while creating file %v. Caused by: %v", path, e)
		responseWriter.WriteHeader(500)
		return
	}
	_, e = io.Copy(file, src)
	if e != nil {
		log.Printf("Error while writing file %v. Caused by: %v", path, e)
		responseWriter.WriteHeader(500)
		return
	}
	responseWriter.WriteHeader(201)
}
