package decorator

import (
	"fmt"
	"io"

	"github.com/petergtz/bitsgo/routes"
)

type Blobstore interface {
	// Can't do the following until it is added in Go: (See also https://github.com/golang/go/issues/6977)
	// routes.Blobstore
	// routes.NoRedirectBlobstore

	// Instead doing:
	routes.Blobstore
	Get(path string) (body io.ReadCloser, err error)
	Put(path string, src io.ReadSeeker) (err error)
}

func ForBlobstoreWithPathPartitioning(delegate Blobstore) *PartitioningPathBlobstoreDecorator {
	return &PartitioningPathBlobstoreDecorator{delegate}
}

type PartitioningPathBlobstoreDecorator struct {
	delegate Blobstore
}

func (decorator *PartitioningPathBlobstoreDecorator) Exists(path string) (bool, error) {
	return decorator.delegate.Exists(pathFor(path))
}

func (decorator *PartitioningPathBlobstoreDecorator) HeadOrDirectToGet(path string) (redirectLocation string, err error) {
	return decorator.delegate.HeadOrDirectToGet(pathFor(path))
}

func (decorator *PartitioningPathBlobstoreDecorator) Get(path string) (body io.ReadCloser, err error) {
	return decorator.delegate.Get(pathFor(path))
}

func (decorator *PartitioningPathBlobstoreDecorator) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	return decorator.delegate.GetOrRedirect(pathFor(path))
}

func (decorator *PartitioningPathBlobstoreDecorator) Put(path string, src io.ReadSeeker) error {
	return decorator.delegate.Put(pathFor(path), src)
}

func (decorator *PartitioningPathBlobstoreDecorator) PutOrRedirect(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	return decorator.delegate.PutOrRedirect(pathFor(path), src)
}

func (decorator *PartitioningPathBlobstoreDecorator) Copy(src, dest string) error {
	return decorator.delegate.Copy(pathFor(src), pathFor(dest))
}

func (decorator *PartitioningPathBlobstoreDecorator) Delete(path string) error {
	return decorator.delegate.Delete(pathFor(path))
}

func (decorator *PartitioningPathBlobstoreDecorator) DeleteDir(prefix string) error {
	if prefix == "" {
		return decorator.delegate.DeleteDir(prefix)
	} else {
		return decorator.delegate.DeleteDir(pathFor(prefix))
	}
}

func pathFor(identifier string) string {
	if len(identifier) >= 4 {
		return fmt.Sprintf("%s/%s/%s", identifier[0:2], identifier[2:4], identifier)
	} else if len(identifier) == 3 {
		return fmt.Sprintf("%s/%s/%s", identifier[0:2], identifier[2:3], identifier)
	} else if len(identifier) == 2 {
		return fmt.Sprintf("%s/%s", identifier[0:2], identifier)
	} else if len(identifier) == 1 {
		return fmt.Sprintf("%s/%s", identifier[0:1], identifier)
	}
	return ""
}

func ForResourceSignerWithPathPartitioning(delegate routes.ResourceSigner) *PartitioningPathResourceSigner {
	return &PartitioningPathResourceSigner{delegate}
}

type PartitioningPathResourceSigner struct {
	delegate routes.ResourceSigner
}

func (signer *PartitioningPathResourceSigner) Sign(resource string, method string) (signedURL string) {
	return signer.delegate.Sign(pathFor(resource), method)
}
