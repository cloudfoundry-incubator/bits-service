package inmemory_blobstore

import (
	"fmt"
	"io"
	"strings"

	"bytes"

	"io/ioutil"

	"github.com/petergtz/bitsgo/routes"
)

type Blobstore struct {
	Entries map[string][]byte
}

func NewBlobstore() *Blobstore {
	return &Blobstore{Entries: make(map[string][]byte)}
}

func NewBlobstoreWithEntries(entries map[string][]byte) *Blobstore {
	return &Blobstore{Entries: entries}
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	_, hasKey := blobstore.Entries[path]
	return hasKey, nil
}

func (blobstore *Blobstore) Head(path string) (redirectLocation string, err error) {
	_, hasKey := blobstore.Entries[path]
	if !hasKey {
		return "", routes.NewNotFoundError()
	}
	return "", nil
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	entry, hasKey := blobstore.Entries[path]
	if !hasKey {
		return nil, "", routes.NewNotFoundError()
	}
	return ioutil.NopCloser(bytes.NewBuffer(entry)), "", nil
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	b, e := ioutil.ReadAll(src)
	if e != nil {
		return "", fmt.Errorf("Error while reading from src %v. Caused by: %v", path, e)
	}
	blobstore.Entries[path] = b
	return "", nil
}

func (blobstore *Blobstore) Copy(src, dest string) (redirectLocation string, err error) {
	blobstore.Entries[dest] = blobstore.Entries[src]
	return
}

func (blobstore *Blobstore) Delete(path string) error {
	_, hasKey := blobstore.Entries[path]
	if !hasKey {
		return routes.NewNotFoundError()
	}
	delete(blobstore.Entries, path)
	return nil
}

func (blobstore *Blobstore) DeletePrefix(prefix string) error {
	for key := range blobstore.Entries {
		if strings.HasPrefix(key, prefix) {
			delete(blobstore.Entries, key)
		}

	}
	return nil
}
