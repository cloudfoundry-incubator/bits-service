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
	Head(path string) (redirectLocation string, err error)
	Get(path string) (body io.ReadCloser, redirectLocation string, err error)
	Put(path string, src io.ReadSeeker) (redirectLocation string, err error)
	Copy(src, dest string) (redirectLocation string, err error)
	Exists(path string) (bool, error)
	Delete(path string) error
	DeleteDir(prefix string) error

	// NoRedirectBlobstore
}

type NoRedirectBlobstore interface {
	GetNoRedirect(path string) (body io.ReadCloser, err error)
	PutNoRedirect(path string, src io.ReadSeeker) (err error)
	// Copy(src, dest string) (err error)
	Exists(path string) (bool, error)
	Delete(path string) error
	DeleteDir(prefix string) error
}
