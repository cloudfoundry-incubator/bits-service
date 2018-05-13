package bitsgo

import (
	"io"
	"io/ioutil"
	"net/http"

	"github.com/petergtz/bitsgo/logger"
)

// Note: this changes the request under certain conditions
func HandleBodySizeLimits(responseWriter http.ResponseWriter, request *http.Request, maxBodySizeLimit uint64) (shouldContinue bool) {
	if maxBodySizeLimit != 0 {
		logger.From(request).Debugw("max-body-size is enabled", "max-body-size", maxBodySizeLimit)
		if request.ContentLength == -1 {
			badRequest(responseWriter, request, "HTTP header does not contain Content-Length")
			return
		}
		if uint64(request.ContentLength) > maxBodySizeLimit {
			defer request.Body.Close()

			// Reading the body here is really just to make Ruby's RestClient happy.
			// For some reason it crashes if we don't read the body.
			io.Copy(ioutil.Discard, request.Body)
			responseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		request.Body = &limitedReader{request.Body, request.ContentLength}
	}
	shouldContinue = true
	return
}

// Copied more or less from io.LimitedReader
type limitedReader struct {
	delegate          io.Reader
	maxBytesRemaining int64
}

func (l *limitedReader) Read(p []byte) (n int, err error) {
	if l.maxBytesRemaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > l.maxBytesRemaining {
		p = p[0:l.maxBytesRemaining]
	}
	n, err = l.delegate.Read(p)
	l.maxBytesRemaining -= int64(n)
	return
}

func (l *limitedReader) Close() error {
	// Reading the body here is really just to make Ruby's RestClient happy.
	// For some reason it crashes if we don't read the body.
	io.Copy(ioutil.Discard, l.delegate)
	// TODO Should we return errors?
	return nil
}
