package middlewares

import "net/http"

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
			responseWriter.WriteHeader(http.StatusUnauthorized)
			return
		}
		middleware.basicAuthHeaderMissingHandler.ServeHTTP(responseWriter, request)
		return
	}

	if !middleware.authorized(username, password) {
		if middleware.unauthorizedHandler == nil {
			responseWriter.WriteHeader(http.StatusUnauthorized)
			return
		}
		middleware.unauthorizedHandler.ServeHTTP(responseWriter, request)
	}
	next(responseWriter, request)
}

func (middleware *BasicAuthMiddleware) authorized(username, password string) bool {
	for _, credential := range middleware.credentials {
		if username == credential.Username && password == credential.Password {
			return true
		}
	}
	return false
}
