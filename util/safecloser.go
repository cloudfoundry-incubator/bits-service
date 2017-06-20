package util

import "io"

type SafeCloser bool

func (safeCloser *SafeCloser) Close(closer io.Closer) error {
	if *safeCloser {
		return nil
	}
	*safeCloser = true
	return closer.Close()
}
