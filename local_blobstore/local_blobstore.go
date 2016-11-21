package local_blobstore

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type LocalBlobstore struct {
	pathPrefix string
}

func NewLocalBlobstore(pathPrefix string) *LocalBlobstore {
	return &LocalBlobstore{pathPrefix: pathPrefix}
}

func (blobstore *LocalBlobstore) Exists(path string) bool {
	_, err := os.Stat(filepath.Join(blobstore.pathPrefix, path))
	return err == nil
}
func (blobstore *LocalBlobstore) Get(path string, responseWriter http.ResponseWriter) {
	file, e := os.Open(filepath.Join(blobstore.pathPrefix, path))

	if os.IsNotExist(e) {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}
	if e != nil {
		responseWriter.WriteHeader(http.StatusInternalServerError)
		log.Printf("Error while opening file %v. Caused by: %v", path, e)
		return
	}
	io.Copy(responseWriter, file)
}

func (blobstore *LocalBlobstore) Put(path string, src io.ReadSeeker, responseWriter http.ResponseWriter) {
	e := os.MkdirAll(filepath.Dir(filepath.Join(blobstore.pathPrefix, path)), os.ModeDir|0755)
	if e != nil {
		log.Printf("Error while creating directories for %v. Caused by: %v", path, e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	file, e := os.Create(filepath.Join(blobstore.pathPrefix, path))
	defer file.Close()
	if e != nil {
		log.Printf("Error while creating file %v. Caused by: %v", path, e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, e = io.Copy(file, src)
	if e != nil {
		log.Printf("Error while writing file %v. Caused by: %v", path, e)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	responseWriter.WriteHeader(http.StatusCreated)
}
