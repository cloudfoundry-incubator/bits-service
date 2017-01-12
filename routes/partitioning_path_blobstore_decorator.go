package routes

import (
	"fmt"
	"io"
)

func DecorateWithPartitioningPathBlobstore(delegate Blobstore) *PartitioningPathBlobstoreDecorator {
	return &PartitioningPathBlobstoreDecorator{delegate}
}

type PartitioningPathBlobstoreDecorator struct {
	delegate Blobstore
}

func (decorator *PartitioningPathBlobstoreDecorator) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	return decorator.delegate.Get(pathFor(path))
}

func (decorator *PartitioningPathBlobstoreDecorator) Head(path string) (redirectLocation string, err error) {
	return decorator.delegate.Head(pathFor(path))
}

func (decorator *PartitioningPathBlobstoreDecorator) Put(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	return decorator.delegate.Put(pathFor(path), src)
}

func (decorator *PartitioningPathBlobstoreDecorator) Exists(path string) (bool, error) {
	return decorator.delegate.Exists(pathFor(path))
}

func (decorator *PartitioningPathBlobstoreDecorator) Delete(path string) error {
	return decorator.delegate.Delete(pathFor(path))
}

func pathFor(identifier string) string {
	return fmt.Sprintf("%s/%s/%s", identifier[0:2], identifier[2:4], identifier)
}

func DecorateWithPartitioningPathResourceSigner(delegate ResourceSigner) *PartitioningPathResourceSigner {
	return &PartitioningPathResourceSigner{delegate}
}

type PartitioningPathResourceSigner struct {
	delegate ResourceSigner
}

func (signer *PartitioningPathResourceSigner) Sign(resource string, method string) (signedURL string) {
	return signer.delegate.Sign(pathFor(resource), method)
}
