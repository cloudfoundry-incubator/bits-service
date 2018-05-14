package middlewares

import (
	"net/http"

	"github.com/petergtz/bitsgo/logger"
)

// This is actually implemented in golang's formdata.go#readForm. But it has bug. Working around it here.
// TODO: reference Github issue here when it is available.
type RemoveTempFilesMiddleware struct{}

func (m *RemoveTempFilesMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	next(responseWriter, request)

	if request.MultipartForm != nil {
		e := request.MultipartForm.RemoveAll()
		if e != nil {
			logger.From(request).Errorw("Could not delete multipart temporary files", "error", e)
		}
	}
}
