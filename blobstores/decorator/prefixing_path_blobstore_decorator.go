package decorator

import (
	"io"

	"github.com/petergtz/bitsgo/routes"
)

func ForBlobstoreWithPathPrefixing(delegate routes.Blobstore, prefix string) *PrefixingPathBlobstoreDecorator {
	return &PrefixingPathBlobstoreDecorator{delegate, prefix}
}

type PrefixingPathBlobstoreDecorator struct {
	delegate routes.Blobstore
	prefix   string
}

func (decorator *PrefixingPathBlobstoreDecorator) Get(path string) (body io.ReadCloser, redirectLocation string, err error) {
	return decorator.delegate.Get(decorator.prefix + path)
}

func (decorator *PrefixingPathBlobstoreDecorator) Head(path string) (redirectLocation string, err error) {
	return decorator.delegate.Head(decorator.prefix + path)
}

func (decorator *PrefixingPathBlobstoreDecorator) Put(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	return decorator.delegate.Put(decorator.prefix+path, src)
}

func (decorator *PrefixingPathBlobstoreDecorator) Copy(src, dest string) (redirectLocation string, err error) {
	return decorator.delegate.Copy(decorator.prefix+src, decorator.prefix+dest)
}

func (decorator *PrefixingPathBlobstoreDecorator) Exists(path string) (bool, error) {
	return decorator.delegate.Exists(decorator.prefix + path)
}

func (decorator *PrefixingPathBlobstoreDecorator) Delete(path string) error {
	return decorator.delegate.Delete(decorator.prefix + path)
}

func (decorator *PrefixingPathBlobstoreDecorator) DeleteDir(prefix string) error {
	return decorator.delegate.DeleteDir(decorator.prefix + prefix)
}

func ForResourceSignerWithPathPrefixing(delegate routes.ResourceSigner, prefix string) *PrefixingPathResourceSigner {
	return &PrefixingPathResourceSigner{delegate, prefix}
}

type PrefixingPathResourceSigner struct {
	delegate routes.ResourceSigner
	prefix   string
}

func (signer *PrefixingPathResourceSigner) Sign(resource string, method string) (signedURL string) {
	return signer.delegate.Sign(signer.prefix+resource, method)
}
