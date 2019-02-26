package middlewares

import (
	"net/http"
	"strings"

	"github.com/cloudfoundry-incubator/bits-service/util"

	"github.com/cloudfoundry-incubator/bits-service/logger"
)

type MultipartMiddleware struct{}

// This middleware is needed, because changing request contexts while passing request along different handlers creates
// new requests objects. So if we only call request.ParseMultipartForm at in the last handler, only that copy of the request contains
// the information about the temp files. By the time all the handlers return, and the server calls finishRequest(), that request
// does not contain the information about the temp files.
func (m *MultipartMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	if strings.Contains(request.Header.Get("Content-Type"), "multipart/form-data") {
		e := request.ParseMultipartForm(32 << 20)
		// lack of formalized error handling in the standard library forces us to do this fragile error string comparison
		// to see if it's a request problem, or if there is some generic error on server side.
		// We'll have to see if we need more special casing around this.
		if e != nil && e.Error() == "multipart: NextPart: unexpected EOF" {
			logger.From(request).Infow("Could not parse multipart", "error", e)
			responseWriter.WriteHeader(http.StatusBadRequest)
			return
		}

		util.PanicOnError(e)

		defer func() {
			if request.MultipartForm != nil {
				e := request.MultipartForm.RemoveAll()
				if e != nil {
					logger.From(request).Errorw("Could not delete multipart temporary files", "error", e)
				}
			}
		}()
	}

	next(responseWriter, request)
}
