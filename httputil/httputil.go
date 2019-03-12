package httputil

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

type Request struct {
	http.Request
}

func (request Request) Build() *http.Request {
	return &request.Request
}

func NewRequest(method, urlStr string, body io.Reader) *Request {
	request, e := http.NewRequest(method, urlStr, body)
	if e != nil {
		panic(e)
	}
	return &Request{*request}
}

func (request *Request) WithBasicAuth(username, password string) *Request {
	request.SetBasicAuth(username, password)
	return request
}

func (request *Request) WithHeader(key, value string) *Request {
	request.Header.Add(key, value)
	return request
}

func NewPutRequest(url string, formFiles map[string]map[string]io.Reader) (*http.Request, error) {
	bodyBuf := &bytes.Buffer{}
	contentType, e := AddFormFileTo(bodyBuf, formFiles)
	if e != nil {
		return nil, errors.Wrapf(e, "url=%v", url)
	}
	request, e := http.NewRequest("PUT", url, bodyBuf)
	if e != nil {
		return nil, errors.Wrapf(e, "url=%v", url)
	}
	request.Header.Add("Content-Type", contentType)
	return request, nil
}

func NewPostRequest(url string, formFiles map[string]map[string]io.Reader) (*http.Request, error) {
	bodyBuf := &bytes.Buffer{}
	contentType, e := AddFormFileTo(bodyBuf, formFiles)
	if e != nil {
		return nil, errors.Wrapf(e, "url=%v", url)
	}
	request, e := http.NewRequest("POST", url, bodyBuf)
	if e != nil {
		return nil, errors.Wrapf(e, "url=%v", url)
	}
	request.Header.Add("Content-Type", contentType)
	return request, nil
}

func AddFormFileTo(body io.Writer, formFiles map[string]map[string]io.Reader) (contentType string, err error) {
	multipartWriter := multipart.NewWriter(body)
	for name, filenameAndReader := range formFiles {
		for filename, reader := range filenameAndReader {
			formFileWriter, e := multipartWriter.CreateFormFile(name, filename)
			if e != nil {
				err = fmt.Errorf("Could not CreateFormFile with name %v and filename %v", name, filename)
				return
			}
			io.Copy(formFileWriter, reader)
		}
	}
	multipartWriter.Close()
	contentType = multipartWriter.FormDataContentType()
	return
}

func MustParse(rawUrl string) *url.URL {
	u, e := url.ParseRequestURI(rawUrl)
	if e != nil {
		panic(e)
	}
	return u
}
