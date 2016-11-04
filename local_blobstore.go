package main

import (
	"fmt"
	"io"
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
func (blobstore *LocalBlobStore) Get(path string) (io.Reader, error) {
	file, e := os.Open(filepath.Join(blobstore.pathPrefix, path))

	if os.IsNotExist(e) {
		return nil, e
	}
	if e != nil {
		return nil, fmt.Errorf("Error while opening file %v. Caused by: %v", path, e)
	}
	return file, nil
}

func (blobstore *LocalBlobStore) Put(path string, src io.Reader) error {
	e := os.MkdirAll(filepath.Dir(filepath.Join(blobstore.pathPrefix, path)), os.ModeDir|0755)
	if e != nil {
		return fmt.Errorf("Error while creating directories for %v. Caused by: %v", path, e)
	}
	file, e := os.Create(filepath.Join(blobstore.pathPrefix, path))
	defer file.Close()
	if e != nil {
		return fmt.Errorf("Error while creating file %v. Caused by: %v", path, e)
	}
	_, e = io.Copy(file, src)
	if e != nil {
		return fmt.Errorf("Error while writing file %v. Caused by: %v", path, e)
	}
	return nil
}

func (blobstore *LocalBlobStore) Delete(path string) error {
	// TODO
	return nil
}
