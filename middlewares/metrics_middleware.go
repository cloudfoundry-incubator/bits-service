package middlewares

import (
	"fmt"
	"io"
	"net/http"

	"github.com/urfave/negroni"

	"os"
	"time"

	"github.com/pkg/errors"
)

type MetricsMiddleware struct {
	writer io.Writer
}

func NewMetricsMiddlewareFromFilename(filename string) *MetricsMiddleware {
	fileWriter, e := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0664)
	if e != nil {
		panic(errors.Wrapf(e, "Could not open file '%v' for appending", filename))
	}
	return &MetricsMiddleware{writer: fileWriter}
}

func NewMetricsMiddleware(writer io.Writer) *MetricsMiddleware {
	return &MetricsMiddleware{writer: writer}
}

func (middleware *MetricsMiddleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	startTime := time.Now()
	negroniResponseWriter, ok := responseWriter.(negroni.ResponseWriter)
	if !ok {
		negroniResponseWriter = negroni.NewResponseWriter(responseWriter)
	}

	next(negroniResponseWriter, request)

	fmt.Fprintf(middleware.writer, "%v %v %v;%v;%v\n",
		request.Method, request.URL.Path, request.Proto, time.Since(startTime).Seconds(), negroniResponseWriter.Size())
}
