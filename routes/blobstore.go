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

type NoSpaceLeftError struct {
	error
}

func NewNoSpaceLeftError() *NotFoundError {
	return &NotFoundError{fmt.Errorf("NoSpaceLeftError")}
}

type Blobstore interface {
	// returns a NotFoundError when the path doesn't exist.
	Exists(path string) (bool, error)
	HeadOrRedirectAsGet(path string) (redirectLocation string, err error)
	GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error)
	PutOrRedirect(path string, src io.ReadSeeker) (redirectLocation string, err error)
	Copy(src, dest string) error
	Delete(path string) error
	DeleteDir(prefix string) error
}

type NoRedirectBlobstore interface {
	Exists(path string) (bool, error)
	Get(path string) (body io.ReadCloser, err error)
	Put(path string, src io.ReadSeeker) (err error)
	Delete(path string) error
	DeleteDir(prefix string) error
}
