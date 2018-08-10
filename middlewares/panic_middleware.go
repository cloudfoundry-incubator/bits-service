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
	RequestID     int64  `json:"request-id"`
}

func (middleware *PanicMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	defer func() {
		if e := recover(); e != nil {
			logger.From(request).Errorw("Internal Server Error.", "error", fmt.Sprintf("%+v", e))
			responseWriter.WriteHeader(http.StatusInternalServerError)
			body, e := json.Marshal(internalServerErrorResponseBody{
				Service:       "Bits-Service",
				Error:         "Internal Server Error",
				VcapRequestID: safeGetStringValueFrom(request.Context(), "vcap-request-id"),
				RequestID:     safeGetInt64ValueFrom(request.Context(), "request-id"),
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

func safeGetStringValueFrom(c context.Context, key string) string {
	if c.Value(key) == nil {
		return ""
	}
	if value, ok := c.Value(key).(string); ok {
		return value
	}
	return ""
}

func safeGetInt64ValueFrom(c context.Context, key string) int64 {
	if c.Value(key) == nil {
		return 0
	}
	if value, ok := c.Value(key).(int64); ok {
		return value
	}
	return 0
}
