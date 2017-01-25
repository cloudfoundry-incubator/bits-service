package middlewares

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

func NewBodySizeLimitMiddleware(bodySizeLimit uint64) *BodySizeLimitMiddleware {
	return &BodySizeLimitMiddleware{bodySizeLimit: bodySizeLimit}
}

type BodySizeLimitMiddleware struct {
	bodySizeLimit uint64
}

func (middleware *BodySizeLimitMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	if middleware.bodySizeLimit != 0 {
		if request.ContentLength == -1 {
			responseWriter.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(responseWriter, "Please provide the header field Content-Length")
			return
		}
		if uint64(request.ContentLength) > middleware.bodySizeLimit {

			// Reading the body here is really just to make Ruby's RestClient happy.
			// For some reason it crashes if we don't read the body.
			defer request.Body.Close()
			io.Copy(ioutil.Discard, request.Body)

			responseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
	}
	next(responseWriter, request)
}
