package util

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func FprintDescriptionAndCodeAsJSON(responseWriter http.ResponseWriter, code int, description string, a ...interface{}) {
	fmt.Fprintf(responseWriter, DescriptionAndCodeAsJSON(code, description, a...))
}

func DescriptionAndCodeAsJSON(code int, description string, a ...interface{}) string {

	m, e := json.Marshal(struct {
		Description string `json:"description"`
		Code        int    `json:"code"`
	}{
		Description: fmt.Sprintf(description, a...),
		Code:        code,
	})
	PanicOnError(e)
	return string(m)
}

func FprintDescriptionAsJSON(responseWriter http.ResponseWriter, description string, a ...interface{}) {
	fmt.Fprintf(responseWriter, DescriptionAsJSON(description, a...))
}

func DescriptionAsJSON(description string, a ...interface{}) string {
	m, e := json.Marshal(struct {
		Description string `json:"description"`
	}{
		Description: fmt.Sprintf(description, a...),
	})
	PanicOnError(e)
	return string(m)
}
