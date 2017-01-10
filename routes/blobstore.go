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
	Exists(path string) (bool, error)
	Delete(path string) error
}
