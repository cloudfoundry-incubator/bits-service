package middlewares

import (
	"net/http"

	"github.com/urfave/negroni"
)

func GorillaMiddlewareFrom(negroniHandler negroni.Handler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			negroniHandler.ServeHTTP(responseWriter, request, next.ServeHTTP)
		})
	}
}
