package decorator

import (
	"io"
	"time"

	"github.com/cloudfoundry-incubator/bits-service"
)

type PrefixingPathBlobstoreDecorator struct {
	delegate bitsgo.Blobstore
	prefix   string
}

func ForBlobstoreWithPathPrefixing(delegate bitsgo.Blobstore, prefix string) *PrefixingPathBlobstoreDecorator {
	return &PrefixingPathBlobstoreDecorator{delegate, prefix}
}

func (decorator *PrefixingPathBlobstoreDecorator) Exists(path string) (bool, error) {
	return decorator.delegate.Exists(decorator.prefix + path)
}

func (decorator *PrefixingPathBlobstoreDecorator) Get(path string) (body io.ReadCloser, err error) {
	return decorator.delegate.Get(decorator.prefix + path)
}

func (decorator *PrefixingPathBlobstoreDecorator) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	return decorator.delegate.GetOrRedirect(decorator.prefix + path)
}

func (decorator *PrefixingPathBlobstoreDecorator) Put(path string, src io.ReadSeeker) error {
	return decorator.delegate.Put(decorator.prefix+path, src)
}

func (decorator *PrefixingPathBlobstoreDecorator) Copy(src, dest string) error {
	return decorator.delegate.Copy(decorator.prefix+src, decorator.prefix+dest)
}

func (decorator *PrefixingPathBlobstoreDecorator) Delete(path string) error {
	return decorator.delegate.Delete(decorator.prefix + path)
}

func (decorator *PrefixingPathBlobstoreDecorator) DeleteDir(prefix string) error {
	return decorator.delegate.DeleteDir(decorator.prefix + prefix)
}

type PrefixingPathResourceSigner struct {
	delegate bitsgo.ResourceSigner
	prefix   string
}

func ForResourceSignerWithPathPrefixing(delegate bitsgo.ResourceSigner, prefix string) *PrefixingPathResourceSigner {
	return &PrefixingPathResourceSigner{delegate, prefix}
}

func (signer *PrefixingPathResourceSigner) Sign(resource string, method string, expirationTime time.Time) (signedURL string) {
	return signer.delegate.Sign(signer.prefix+resource, method, expirationTime)
}
