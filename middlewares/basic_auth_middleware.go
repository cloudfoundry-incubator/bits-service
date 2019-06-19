package middlewares

import (
	"crypto/subtle"
	"net/http"
)

type Credential struct {
	Username, Password string
}

type BasicAuthMiddleware struct {
	credentials                   []Credential
	basicAuthHeaderMissingHandler http.Handler
	unauthorizedHandler           http.Handler
}

func NewBasicAuthMiddleWare(credentials ...Credential) *BasicAuthMiddleware {
	return &BasicAuthMiddleware{credentials: credentials}
}

func (middleware *BasicAuthMiddleware) WithBasicAuthHeaderMissingHandler(handler http.Handler) *BasicAuthMiddleware {
	middleware.basicAuthHeaderMissingHandler = handler
	return middleware
}

func (middleware *BasicAuthMiddleware) WithUnauthorizedHandler(handler http.Handler) *BasicAuthMiddleware {
	middleware.unauthorizedHandler = handler
	return middleware
}

func (middleware *BasicAuthMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	username, password, ok := request.BasicAuth()
	if !ok {
		if middleware.basicAuthHeaderMissingHandler == nil {
			responseWriter.Header().Set("WWW-Authenticate", `Basic realm="bits-service"`)
			responseWriter.WriteHeader(http.StatusUnauthorized)
			return
		}
		middleware.basicAuthHeaderMissingHandler.ServeHTTP(responseWriter, request)
		return
	}

	if !middleware.authorized(username, password) {
		if middleware.unauthorizedHandler == nil {
			responseWriter.Header().Set("WWW-Authenticate", `Basic realm="bits-service"`)
			responseWriter.WriteHeader(http.StatusUnauthorized)
			return
		}
		middleware.unauthorizedHandler.ServeHTTP(responseWriter, request)
	}
	next(responseWriter, request)
}

func (middleware *BasicAuthMiddleware) authorized(username, password string) bool {
	for _, credential := range middleware.credentials {
		if subtle.ConstantTimeCompare([]byte(username), []byte(credential.Username)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(credential.Password)) == 1 {
			return true
		}
	}
	return false
}
