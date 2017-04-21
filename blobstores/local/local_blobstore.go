package local

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"syscall"

	"github.com/petergtz/bitsgo/logger"
	"github.com/petergtz/bitsgo/routes"
	"github.com/pkg/errors"
	"github.com/uber-go/zap"
)

type Blobstore struct {
	pathPrefix string
}

func NewBlobstore(pathPrefix string) *Blobstore {
	return &Blobstore{pathPrefix: pathPrefix}
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	_, err := os.Stat(filepath.Join(blobstore.pathPrefix, path))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("Could not stat on %v. Caused by: %v", filepath.Join(blobstore.pathPrefix, path), err)
	}
	return true, nil
}

func (blobstore *Blobstore) HeadOrRedirectAsGet(path string) (redirectLocation string, err error) {
	logger.Log.Debug("Head", zap.String("local-path", filepath.Join(blobstore.pathPrefix, path)))
	_, e := os.Stat(filepath.Join(blobstore.pathPrefix, path))

	if os.IsNotExist(e) {
		return "", routes.NewNotFoundError()
	}
	if e != nil {
		return "", fmt.Errorf("Error while opening file %v. Caused by: %v", path, e)
	}
	return "", nil
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	logger.Log.Debug("GetNoRedirect", zap.String("local-path", filepath.Join(blobstore.pathPrefix, path)))
	file, e := os.Open(filepath.Join(blobstore.pathPrefix, path))

	if os.IsNotExist(e) {
		return nil, routes.NewNotFoundError()
	}
	if e != nil {
		return nil, fmt.Errorf("Error while opening file %v. Caused by: %v", path, e)
	}
	return file, nil
}

func (blobstore *Blobstore) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	body, e := blobstore.Get(path)
	return body, "", e
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	e := os.MkdirAll(filepath.Dir(filepath.Join(blobstore.pathPrefix, path)), os.ModeDir|0755)
	if e != nil {
		return fmt.Errorf("Error while creating directories for %v. Caused by: %v", path, e)
	}
	file, e := os.Create(filepath.Join(blobstore.pathPrefix, path))
	if e != nil {
		if e.(*os.PathError).Err == syscall.ENOSPC {
			return routes.NewNoSpaceLeftError()
		}
		return fmt.Errorf("Error while creating file %v. Caused by: %v", path, e)
	}
	defer file.Close()
	_, e = io.Copy(file, src)
	if e != nil {
		if e.(*os.PathError).Err == syscall.ENOSPC {
			return routes.NewNoSpaceLeftError()
		}
		return fmt.Errorf("Error while writing file %v. Caused by: %v", path, e)
	}
	return nil
}

func (blobstore *Blobstore) PutOrRedirect(path string, src io.ReadSeeker) (redirectLocation string, err error) {
	return "", blobstore.Put(path, src)
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	srcFull := filepath.Join(blobstore.pathPrefix, src)
	destFull := filepath.Join(blobstore.pathPrefix, dest)

	srcFile, e := os.Open(srcFull)
	if e != nil {
		if os.IsNotExist(e) {
			return routes.NewNotFoundError()
		}
		return errors.Wrapf(e, "Opening src failed. (src=%v, dest=%v)", srcFull, destFull)
	}
	defer srcFile.Close()

	e = os.MkdirAll(filepath.Dir(destFull), 0755)
	if e != nil {
		return errors.Wrapf(e, "Make dir failed. (src=%v, dest=%v)", srcFull, destFull)
	}

	destFile, e := os.Create(destFull)
	if e != nil {
		return errors.Wrapf(e, "Creating dest failed. (src=%v, dest=%v)", srcFull, destFull)
	}
	defer destFile.Close()

	_, e = io.Copy(destFile, srcFile)
	if e != nil {
		return errors.Wrapf(e, "Copying failed. (src=%v, dest=%v)", srcFull, destFull)
	}

	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	_, e := os.Stat(filepath.Join(blobstore.pathPrefix, path))
	if os.IsNotExist(e) {
		return routes.NewNotFoundError()
	}
	e = os.RemoveAll(filepath.Join(blobstore.pathPrefix, path))
	if e != nil {
		return fmt.Errorf("Error while deleting file %v. Caused by: %v", path, e)
	}
	return nil
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	e := os.RemoveAll(filepath.Join(blobstore.pathPrefix, prefix))
	if e != nil {
		return errors.Wrapf(e, "Failed to delete path %v", filepath.Join(blobstore.pathPrefix, prefix))
	}
	return nil
}
