package httputil

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
)

func AddFormFileTo(body io.Writer, formFiles map[string]map[string]io.Reader) (header http.Header, err error) {
	header = make(map[string][]string)
	for name, fileAndReader := range formFiles {
		multipartWriter := multipart.NewWriter(body)
		for file, reader := range fileAndReader {
			formFileWriter, e := multipartWriter.CreateFormFile(name, file)
			if e != nil {
				err = fmt.Errorf("Could not CreateFormFile with name %v and filename %v", name, file)
				return
			}
			io.Copy(formFileWriter, reader)
			multipartWriter.Close()
			header["Content-Type"] = append(header["Content-Type"], multipartWriter.FormDataContentType())
		}
	}
	return
}

func AddHeaderTo(request *http.Request, header http.Header) {
	for key, values := range header {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
}

func MustParse(rawUrl string) *url.URL {
	u, e := url.ParseRequestURI(rawUrl)
	if e != nil {
		panic(e)
	}
	return u
}
