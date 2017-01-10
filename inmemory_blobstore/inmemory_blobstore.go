package inmemory_blobstore

import (
	"fmt"
	"io"

	"bytes"

	"io/ioutil"

	"github.com/petergtz/bitsgo/routes"
)

type InMemoryBlobstore struct {
	Entries map[string][]byte
}

func NewInMemoryBlobstore() *InMemoryBlobstore {
	return &InMemoryBlobstore{Entries: make(map[string][]byte)}
}

func NewInMemoryBlobstoreWithEntries(entries map[string][]byte) *InMemoryBlobstore {
	return &InMemoryBlobstore{Entries: entries}
}

func (blobstore *InMemoryBlobstore) Exists(path string) (bool, error) {
	_, hasKey := blobstore.Entries[path]
	return hasKey, nil
}

func (blobstore *InMemoryBlobstore) Head(path string) (redirectLocation string, err error) {
	_, hasKey := blobstore.Entries[path]
	if !hasKey {
		return "", routes.NewNotFoundError()
	}
	return "", nil
}

func (blobstore *InMemoryBlobstore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	entry, hasKey := blobstore.Entries[path]
	if !hasKey {
		return nil, "", routes.NewNotFoundError()
	}
	return ioutil.NopCloser(bytes.NewBuffer(entry)), "", nil
}

func (blobstore *InMemoryBlobstore) Put(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	b, e := ioutil.ReadAll(src)
	if e != nil {
		return "", fmt.Errorf("Error while reading from src %v. Caused by: %v", path, e)
	}
	blobstore.Entries[path] = b
	return "", nil
}

func (blobstore *InMemoryBlobstore) Delete(path string) error {
	_, hasKey := blobstore.Entries[path]
	if !hasKey {
		return routes.NewNotFoundError()
	}
	delete(blobstore.Entries, path)
	return nil
}
