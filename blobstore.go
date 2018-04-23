package bitsgo

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

func NewNoSpaceLeftError() *NoSpaceLeftError {
	return &NoSpaceLeftError{fmt.Errorf("NoSpaceLeftError")}
}

type Blobstore interface {
	Exists(path string) (bool, error)
	HeadOrRedirectAsGet(path string) (redirectLocation string, err error)

	// Implementers must return *NotFoundError when the resource cannot be found
	GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error)

	// Implementers must return *NoSpaceLeftError when there's no space left on device.
	Put(path string, src io.ReadSeeker) error
	Copy(src, dest string) error
	Delete(path string) error
	DeleteDir(prefix string) error
}

type NoRedirectBlobstore interface {
	Exists(path string) (bool, error)

	// Implementers must return *NotFoundError when the resource cannot be found
	Get(path string) (body io.ReadCloser, err error)

	// Implementers must return *NoSpaceLeftError when there's no space left on device.
	Put(path string, src io.ReadSeeker) error
	Delete(path string) error
	DeleteDir(prefix string) error
}
