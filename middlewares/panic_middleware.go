package middlewares

import (
	"fmt"
	"net/http"

	"github.com/petergtz/bitsgo/logger"
)

type PanicMiddleware struct{}

func (middleware *PanicMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	defer func() {
		if e := recover(); e != nil {
			logger.From(request).Errorw("Internal Server Error.", "error", fmt.Sprintf("%+v", e))
			responseWriter.WriteHeader(http.StatusInternalServerError)
		}
	}()

	next(responseWriter, request)
}
