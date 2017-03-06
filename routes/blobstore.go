package routes

import (
	"fmt"
	"io"
)

type NotFoundError struct {
	error
}

func NewNotFoundError() *NotFoundError {
	return &NotFoundError{fmt.Errorf("NotFoundError")}
}

func NewNotFoundErrorWithMessage(message string) *NotFoundError {
	return &NotFoundError{fmt.Errorf(message)}
}

type Blobstore interface {
	// returns a NotFoundError when the path doesn't exist.
	HeadOrDirectToGet(path string) (redirectLocation string, err error)
	GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error)
	PutOrRedirect(path string, src io.ReadSeeker) (redirectLocation string, err error)
	Copy(src, dest string) error
	Exists(path string) (bool, error)
	Delete(path string) error
	DeleteDir(prefix string) error
}

type NoRedirectBlobstore interface {
	Get(path string) (body io.ReadCloser, err error)
	Put(path string, src io.ReadSeeker) (err error)
	Exists(path string) (bool, error)
	Delete(path string) error
	DeleteDir(prefix string) error
}
