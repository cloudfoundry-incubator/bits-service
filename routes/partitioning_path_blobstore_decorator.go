package routes

import "io"

type PartitioningPathBlobstoreDecorator struct {
	delegate Blobstore
}

func (decorator *PartitioningPathBlobstoreDecorator) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	return decorator.delegate.Get(pathFor(path))
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
