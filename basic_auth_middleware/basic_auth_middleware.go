package basic_auth_middleware

import "net/http"

type BasicAuthMiddleware struct {
	Username, Password string
}

// TODO this middleware should be configurable with a custom ForbiddenHandler

func (middleware *BasicAuthMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	username, password, ok := request.BasicAuth()
	if !ok || username != middleware.Username || password != middleware.Password {
		responseWriter.WriteHeader(http.StatusForbidden)
		return
	}
	next(responseWriter, request)
}
