package local

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petergtz/bitsgo/config"

	"syscall"

	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/logger"
	"github.com/pkg/errors"
)

type Blobstore struct {
	pathPrefix string
}

func NewBlobstore(localConfig config.LocalBlobstoreConfig) *Blobstore {
	return &Blobstore{pathPrefix: localConfig.PathPrefix}
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
	logger.Log.Debugw("Head", "local-path", filepath.Join(blobstore.pathPrefix, path))
	_, e := os.Stat(filepath.Join(blobstore.pathPrefix, path))

	if os.IsNotExist(e) {
		return "", bitsgo.NewNotFoundError()
	}
	if e != nil {
		return "", fmt.Errorf("Error while opening file %v. Caused by: %v", path, e)
	}
	return "", nil
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	logger.Log.Debugw("GetNoRedirect", "local-path", filepath.Join(blobstore.pathPrefix, path))
	file, e := os.Open(filepath.Join(blobstore.pathPrefix, path))

	if os.IsNotExist(e) {
		return nil, bitsgo.NewNotFoundError()
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
	if e, isPathError := e.(*os.PathError); isPathError && e.Err == syscall.ENOSPC {
		return bitsgo.NewNoSpaceLeftError()
	}
	if e != nil {
		return fmt.Errorf("Error while creating directories for %v. Caused by: %v", path, e)
	}
	file, e := os.Create(filepath.Join(blobstore.pathPrefix, path))
	if e, isPathError := e.(*os.PathError); isPathError && e.Err == syscall.ENOSPC {
		return bitsgo.NewNoSpaceLeftError()
	}
	if e != nil {
		return fmt.Errorf("Error while creating file %v. Caused by: %v", path, e)
	}
	defer file.Close()
	_, e = io.Copy(file, src)
	if e, isPathError := e.(*os.PathError); isPathError && e.Err == syscall.ENOSPC {
		return bitsgo.NewNoSpaceLeftError()
	}
	if e != nil {
		return fmt.Errorf("Error while writing file %v. Caused by: %v", path, e)
	}
	return nil
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	srcFull := filepath.Join(blobstore.pathPrefix, src)
	destFull := filepath.Join(blobstore.pathPrefix, dest)

	srcFile, e := os.Open(srcFull)
	if e, isPathError := e.(*os.PathError); isPathError && e.Err == syscall.ENOSPC {
		return bitsgo.NewNoSpaceLeftError()
	}
	if os.IsNotExist(e) {
		return bitsgo.NewNotFoundError()
	}
	if e != nil {
		return errors.Wrapf(e, "Opening src failed. (src=%v, dest=%v)", srcFull, destFull)
	}
	defer srcFile.Close()

	e = os.MkdirAll(filepath.Dir(destFull), 0755)
	if e, isPathError := e.(*os.PathError); isPathError && e.Err == syscall.ENOSPC {
		return bitsgo.NewNoSpaceLeftError()
	}
	if e != nil {
		return errors.Wrapf(e, "Make dir failed. (src=%v, dest=%v)", srcFull, destFull)
	}

	destFile, e := os.Create(destFull)
	if e, isPathError := e.(*os.PathError); isPathError && e.Err == syscall.ENOSPC {
		return bitsgo.NewNoSpaceLeftError()
	}
	if e != nil {
		return errors.Wrapf(e, "Creating dest failed. (src=%v, dest=%v)", srcFull, destFull)
	}
	defer destFile.Close()

	_, e = io.Copy(destFile, srcFile)
	if e, isPathError := e.(*os.PathError); isPathError && e.Err == syscall.ENOSPC {
		return bitsgo.NewNoSpaceLeftError()
	}
	if e != nil {
		return errors.Wrapf(e, "Copying failed. (src=%v, dest=%v)", srcFull, destFull)
	}

	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	_, e := os.Stat(filepath.Join(blobstore.pathPrefix, path))
	if e, isPathError := e.(*os.PathError); isPathError && e.Err == syscall.ENOSPC {
		return bitsgo.NewNoSpaceLeftError()
	}
	if os.IsNotExist(e) {
		return bitsgo.NewNotFoundError()
	}
	e = os.RemoveAll(filepath.Join(blobstore.pathPrefix, path))
	if e, isPathError := e.(*os.PathError); isPathError && e.Err == syscall.ENOSPC {
		return bitsgo.NewNoSpaceLeftError()
	}
	if e != nil {
		return fmt.Errorf("Error while deleting file %v. Caused by: %v", path, e)
	}
	return nil
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	e := os.RemoveAll(filepath.Join(blobstore.pathPrefix, prefix))
	if e, isPathError := e.(*os.PathError); isPathError && e.Err == syscall.ENOSPC {
		return bitsgo.NewNoSpaceLeftError()
	}
	if e != nil {
		return errors.Wrapf(e, "Failed to delete path %v", filepath.Join(blobstore.pathPrefix, prefix))
	}
	return nil
}
