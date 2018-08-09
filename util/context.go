package util

import (
	"context"
	"net/http"
)

func RequestWithContextValues(r *http.Request, keysAndValues ...interface{}) *http.Request {
	if len(keysAndValues)%2 != 0 {
		panic("keysAndValues must have an even number of elements")
	}

	c := r.Context()
	for i := 0; i < len(keysAndValues); i += 2 {
		c = context.WithValue(c, keysAndValues[i], keysAndValues[i+1])
	}
	return r.WithContext(c)
}
