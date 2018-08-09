package middlewares

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cloudfoundry-incubator/bits-service/logger"
)

type PanicMiddleware struct{}

type internalServerErrorResponseBody struct {
	Service       string `json:"service"`
	Error         string `json:"error"`
	VcapRequestID string `json:"vcap-request-id"`
	RequestID     string `json:"request-id"`
}

func (middleware *PanicMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	defer func() {
		if e := recover(); e != nil {
			logger.From(request).Errorw("Internal Server Error.", "error", fmt.Sprintf("%+v", e))
			responseWriter.WriteHeader(http.StatusInternalServerError)
			body, e := json.Marshal(internalServerErrorResponseBody{
				Service:       "Bits-Service",
				Error:         "Internal Server Error",
				VcapRequestID: safeGetValueFrom(request.Context(), "vcap-request-id"),
				RequestID:     safeGetValueFrom(request.Context(), "request-id"),
			})
			if e != nil {
				// Nothing we can do at this point
				return
			}
			responseWriter.Write(body)
		}
	}()

	next(responseWriter, request)
}

func safeGetValueFrom(c context.Context, key string) string {
	if c.Value(key) == nil {
		return ""
	}
	if value, ok := c.Value(key).(string); ok {
		return value
	} else {
		return ""
	}
}
